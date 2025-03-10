// Package buildtsi reads an in-memory index and exports it as a TSI index.
package buildtsi

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/influxdata/influxdb/logger"
	"github.com/influxdata/influxdb/models"
	"github.com/influxdata/influxdb/pkg/fs"
	"github.com/influxdata/influxdb/storage"
	"github.com/influxdata/influxdb/storage/wal"
	"github.com/influxdata/influxdb/toml"
	"github.com/influxdata/influxdb/tsdb"
	"github.com/influxdata/influxdb/tsdb/tsi1"
	"github.com/influxdata/influxdb/tsdb/tsm1"
	"go.uber.org/zap"
)

const defaultBatchSize = 10000

// Command represents the program execution for "influx_inspect buildtsi".
type Command struct {
	Stderr  io.Writer
	Stdout  io.Writer
	Verbose bool
	Logger  *zap.Logger

	concurrency     int // Number of goroutines to dedicate to shard index building.
	databaseFilter  string
	retentionFilter string
	shardFilter     string
	maxLogFileSize  int64
	maxCacheSize    uint64
	batchSize       int
}

// NewCommand returns a new instance of Command.
func NewCommand() *Command {
	return &Command{
		Stderr:      os.Stderr,
		Stdout:      os.Stdout,
		Logger:      zap.NewNop(),
		batchSize:   defaultBatchSize,
		concurrency: runtime.GOMAXPROCS(0),
	}
}

// Run executes the command.
func (cmd *Command) Run(args ...string) error {
	fs := flag.NewFlagSet("buildtsi", flag.ExitOnError)
	dataDir := fs.String("datadir", "", "data directory")
	walDir := fs.String("waldir", "", "WAL directory")
	fs.IntVar(&cmd.concurrency, "concurrency", runtime.GOMAXPROCS(0), "Number of workers to dedicate to shard index building. Defaults to GOMAXPROCS")
	fs.StringVar(&cmd.databaseFilter, "database", "", "optional: database name")
	fs.StringVar(&cmd.retentionFilter, "retention", "", "optional: retention policy")
	fs.StringVar(&cmd.shardFilter, "shard", "", "optional: shard id")
	fs.Int64Var(&cmd.maxLogFileSize, "max-log-file-size", tsi1.DefaultMaxIndexLogFileSize, "optional: maximum log file size")
	fs.Uint64Var(&cmd.maxCacheSize, "max-cache-size", uint64(tsm1.DefaultCacheMaxMemorySize), "optional: maximum cache size")
	fs.IntVar(&cmd.batchSize, "batch-size", defaultBatchSize, "optional: set the size of the batches we write to the index. Setting this can have adverse affects on performance and heap requirements")
	fs.BoolVar(&cmd.Verbose, "v", false, "verbose")
	fs.SetOutput(cmd.Stdout)
	if err := fs.Parse(args); err != nil {
		return err
	} else if fs.NArg() > 0 || *dataDir == "" || *walDir == "" {
		fs.Usage()
		return nil
	}
	cmd.Logger = logger.New(cmd.Stderr)

	return cmd.run(*dataDir, *walDir)
}

func (cmd *Command) run(dataDir, walDir string) error {
	// Verify the user actually wants to run as root.
	if isRoot() {
		fmt.Println("You are currently running as root. This will build your")
		fmt.Println("index files with root ownership and will be inaccessible")
		fmt.Println("if you run influxd as a non-root user. You should run")
		fmt.Println("buildtsi as the same user you are running influxd.")
		fmt.Print("Are you sure you want to continue? (y/N): ")
		var answer string
		if fmt.Scanln(&answer); !strings.HasPrefix(strings.TrimSpace(strings.ToLower(answer)), "y") {
			return fmt.Errorf("operation aborted")
		}
	}

	fis, err := ioutil.ReadDir(dataDir)
	if err != nil {
		return err
	}

	for _, fi := range fis {
		name := fi.Name()
		if !fi.IsDir() {
			continue
		} else if cmd.databaseFilter != "" && name != cmd.databaseFilter {
			continue
		}

		if err := cmd.processDatabase(name, filepath.Join(dataDir, name), filepath.Join(walDir, name)); err != nil {
			return err
		}
	}

	return nil
}

func (cmd *Command) processDatabase(dbName, dataDir, walDir string) error {
	cmd.Logger.Info("Rebuilding database", zap.String("name", dbName))

	sfile := tsdb.NewSeriesFile(filepath.Join(dataDir, storage.DefaultSeriesFileDirectoryName))
	sfile.Logger = cmd.Logger
	if err := sfile.Open(context.Background()); err != nil {
		return err
	}
	defer sfile.Close()

	fis, err := ioutil.ReadDir(dataDir)
	if err != nil {
		return err
	}

	for _, fi := range fis {
		rpName := fi.Name()
		if !fi.IsDir() {
			continue
		} else if rpName == storage.DefaultSeriesFileDirectoryName {
			continue
		} else if cmd.retentionFilter != "" && rpName != cmd.retentionFilter {
			continue
		}

		if err := cmd.processRetentionPolicy(sfile, dbName, rpName, filepath.Join(dataDir, rpName), filepath.Join(walDir, rpName)); err != nil {
			return err
		}
	}

	return nil
}

func (cmd *Command) processRetentionPolicy(sfile *tsdb.SeriesFile, dbName, rpName, dataDir, walDir string) error {
	cmd.Logger.Info("Rebuilding retention policy", logger.Database(dbName), logger.RetentionPolicy(rpName))

	fis, err := ioutil.ReadDir(dataDir)
	if err != nil {
		return err
	}

	type shard struct {
		ID   uint64
		Path string
	}

	var shards []shard

	for _, fi := range fis {
		if !fi.IsDir() {
			continue
		} else if cmd.shardFilter != "" && fi.Name() != cmd.shardFilter {
			continue
		}

		shardID, err := strconv.ParseUint(fi.Name(), 10, 64)
		if err != nil {
			continue
		}

		shards = append(shards, shard{shardID, fi.Name()})
	}

	errC := make(chan error, len(shards))
	var maxi uint32 // index of maximum shard being worked on.
	for k := 0; k < cmd.concurrency; k++ {
		go func() {
			for {
				i := int(atomic.AddUint32(&maxi, 1) - 1) // Get next partition to work on.
				if i >= len(shards) {
					return // No more work.
				}

				id, name := shards[i].ID, shards[i].Path
				log := cmd.Logger.With(logger.Database(dbName), logger.RetentionPolicy(rpName), logger.Shard(id))
				errC <- IndexShard(sfile, filepath.Join(dataDir, "index"), filepath.Join(dataDir, name), filepath.Join(walDir, name), cmd.maxLogFileSize, cmd.maxCacheSize, cmd.batchSize, log, cmd.Verbose)
			}
		}()
	}

	// Check for error
	for i := 0; i < cap(errC); i++ {
		if err := <-errC; err != nil {
			return err
		}
	}
	return nil
}

func IndexShard(sfile *tsdb.SeriesFile, indexPath, dataDir, walDir string, maxLogFileSize int64, maxCacheSize uint64, batchSize int, log *zap.Logger, verboseLogging bool) error {
	log.Info("Rebuilding shard")

	// Check if shard already has a TSI index.
	log.Info("Checking index path", zap.String("path", indexPath))
	if _, err := os.Stat(indexPath); !os.IsNotExist(err) {
		log.Info("tsi1 index already exists, skipping", zap.String("path", indexPath))
		return nil
	}

	log.Info("Opening shard")

	// Remove temporary index files if this is being re-run.
	tmpPath := filepath.Join(dataDir, ".index")
	log.Info("Cleaning up partial index from previous run, if any")
	if err := os.RemoveAll(tmpPath); err != nil {
		return err
	}

	// Open TSI index in temporary path.
	c := tsi1.NewConfig()
	c.MaxIndexLogFileSize = toml.Size(maxLogFileSize)

	tsiIndex := tsi1.NewIndex(sfile, c,
		tsi1.WithPath(tmpPath),
		tsi1.DisableFsync(),
		// Each new series entry in a log file is ~12 bytes so this should
		// roughly equate to one flush to the file for every batch.
		tsi1.WithLogFileBufferSize(12*batchSize),
		tsi1.DisableMetrics(), // Disable metrics when rebuilding an index
	)
	tsiIndex.WithLogger(log)

	log.Info("Opening tsi index in temporary location", zap.String("path", tmpPath))
	if err := tsiIndex.Open(context.Background()); err != nil {
		return err
	}
	defer tsiIndex.Close()

	// Write out tsm1 files.
	// Find shard files.
	tsmPaths, err := collectTSMFiles(dataDir)
	if err != nil {
		return err
	}

	log.Info("Iterating over tsm files")
	for _, path := range tsmPaths {
		log.Info("Processing tsm file", zap.String("path", path))
		if err := IndexTSMFile(tsiIndex, path, batchSize, log, verboseLogging); err != nil {
			return err
		}
	}

	// Write out wal files.
	walPaths, err := collectWALFiles(walDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

	} else {
		log.Info("Building cache from wal files")
		cache := tsm1.NewCache(uint64(tsm1.DefaultCacheMaxMemorySize))
		loader := tsm1.NewCacheLoader(walPaths)
		loader.WithLogger(log)
		if err := loader.Load(cache); err != nil {
			return err
		}

		log.Info("Iterating over cache")
		collection := &tsdb.SeriesCollection{
			Keys:  make([][]byte, 0, batchSize),
			Names: make([][]byte, 0, batchSize),
			Tags:  make([]models.Tags, 0, batchSize),
			Types: make([]models.FieldType, 0, batchSize),
		}

		for _, key := range cache.Keys() {
			seriesKey, _ := tsm1.SeriesAndFieldFromCompositeKey(key)
			name, tags := models.ParseKeyBytes(seriesKey)
			typ, _ := cache.Type(key)

			if verboseLogging {
				log.Info("Series", zap.String("name", string(name)), zap.String("tags", tags.String()))
			}

			collection.Keys = append(collection.Keys, seriesKey)
			collection.Names = append(collection.Names, name)
			collection.Tags = append(collection.Tags, tags)
			collection.Types = append(collection.Types, typ)

			// Flush batch?
			if collection.Length() == batchSize {
				if err := tsiIndex.CreateSeriesListIfNotExists(collection); err != nil {
					return fmt.Errorf("problem creating series: (%s)", err)
				}
				collection.Truncate(0)
			}
		}

		// Flush any remaining series in the batches
		if collection.Length() > 0 {
			if err := tsiIndex.CreateSeriesListIfNotExists(collection); err != nil {
				return fmt.Errorf("problem creating series: (%s)", err)
			}
			collection = nil
		}
	}

	// Attempt to compact the index & wait for all compactions to complete.
	log.Info("compacting index")
	tsiIndex.Compact()
	tsiIndex.Wait()

	// Close TSI index.
	log.Info("Closing tsi index")
	if err := tsiIndex.Close(); err != nil {
		return err
	}

	// Rename TSI to standard path.
	log.Info("Moving tsi to permanent location")
	return fs.RenameFile(tmpPath, indexPath)
}

func IndexTSMFile(index *tsi1.Index, path string, batchSize int, log *zap.Logger, verboseLogging bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	r, err := tsm1.NewTSMReader(f)
	if err != nil {
		log.Warn("Unable to read, skipping", zap.String("path", path), zap.Error(err))
		return nil
	}
	defer r.Close()

	collection := &tsdb.SeriesCollection{
		Keys:  make([][]byte, 0, batchSize),
		Names: make([][]byte, 0, batchSize),
		Tags:  make([]models.Tags, batchSize),
		Types: make([]models.FieldType, 0, batchSize),
	}
	var ti int
	iter := r.Iterator(nil)
	for iter.Next() {
		key := iter.Key()
		seriesKey, _ := tsm1.SeriesAndFieldFromCompositeKey(key)
		var name []byte
		name, collection.Tags[ti] = models.ParseKeyBytesWithTags(seriesKey, collection.Tags[ti])
		typ := iter.Type()

		if verboseLogging {
			log.Info("Series", zap.String("name", string(name)), zap.String("tags", collection.Tags[ti].String()))
		}

		collection.Keys = append(collection.Keys, seriesKey)
		collection.Names = append(collection.Names, name)
		collection.Types = append(collection.Types, modelsFieldType(typ))
		ti++

		// Flush batch?
		if len(collection.Keys) == batchSize {
			collection.Truncate(ti)
			if err := index.CreateSeriesListIfNotExists(collection); err != nil {
				return fmt.Errorf("problem creating series: (%s)", err)
			}
			collection.Truncate(0)
			collection.Tags = collection.Tags[:batchSize]
			ti = 0 // Reset tags.
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("problem creating series: (%s)", err)
	}

	// Flush any remaining series in the batches
	if len(collection.Keys) > 0 {
		collection.Truncate(ti)
		if err := index.CreateSeriesListIfNotExists(collection); err != nil {
			return fmt.Errorf("problem creating series: (%s)", err)
		}
	}
	return nil
}

func collectTSMFiles(path string) ([]string, error) {
	fis, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, fi := range fis {
		if filepath.Ext(fi.Name()) != "."+tsm1.TSMFileExtension {
			continue
		}
		paths = append(paths, filepath.Join(path, fi.Name()))
	}
	return paths, nil
}

func collectWALFiles(path string) ([]string, error) {
	if path == "" {
		return nil, os.ErrNotExist
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}
	fis, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, fi := range fis {
		if filepath.Ext(fi.Name()) != "."+wal.WALFileExtension {
			continue
		}
		paths = append(paths, filepath.Join(path, fi.Name()))
	}
	return paths, nil
}

func isRoot() bool {
	user, _ := user.Current()
	return user != nil && user.Username == "root"
}

func modelsFieldType(block byte) models.FieldType {
	switch block {
	case tsm1.BlockFloat64:
		return models.Float
	case tsm1.BlockInteger:
		return models.Integer
	case tsm1.BlockBoolean:
		return models.Boolean
	case tsm1.BlockString:
		return models.String
	case tsm1.BlockUnsigned:
		return models.Unsigned
	default:
		return models.Empty
	}
}
