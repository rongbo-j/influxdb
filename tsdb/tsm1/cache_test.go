package tsm1

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/influxdata/influxdb/storage/wal"

	"github.com/golang/snappy"
)

func TestCache_NewCache(t *testing.T) {
	c := NewCache(100)
	if c == nil {
		t.Fatalf("failed to create new cache")
	}

	if c.MaxSize() != 100 {
		t.Fatalf("new cache max size not correct")
	}
	if c.Size() != 0 {
		t.Fatalf("new cache size not correct")
	}
	if len(c.Keys()) != 0 {
		t.Fatalf("new cache keys not correct: %v", c.Keys())
	}
}

func TestCache_CacheWrite(t *testing.T) {
	v0 := NewValue(1, 1.0)
	v1 := NewValue(2, 2.0)
	v2 := NewValue(3, 3.0)
	values := Values{v0, v1, v2}
	valuesSize := uint64(v0.Size() + v1.Size() + v2.Size())

	c := NewCache(3 * valuesSize)

	if err := c.Write([]byte("foo"), values); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}
	if err := c.Write([]byte("bar"), values); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}
	if n := c.Size(); n != 2*valuesSize+6 {
		t.Fatalf("cache size incorrect after 2 writes, exp %d, got %d", 2*valuesSize, n)
	}

	if exp, keys := [][]byte{[]byte("bar"), []byte("foo")}, c.Keys(); !reflect.DeepEqual(keys, exp) {
		t.Fatalf("cache keys incorrect after 2 writes, exp %v, got %v", exp, keys)
	}
}

func TestCache_CacheWrite_TypeConflict(t *testing.T) {
	v0 := NewValue(1, 1.0)
	v1 := NewValue(2, int(64))
	values := Values{v0, v1}
	valuesSize := v0.Size() + v1.Size()

	c := NewCache(uint64(2 * valuesSize))

	if err := c.Write([]byte("foo"), values[:1]); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}

	if err := c.Write([]byte("foo"), values[1:]); err == nil {
		t.Fatalf("expected field type conflict")
	}

	if exp, got := uint64(v0.Size())+3, c.Size(); exp != got {
		t.Fatalf("cache size incorrect after 2 writes, exp %d, got %d", exp, got)
	}
}

func TestCache_CacheWriteMulti(t *testing.T) {
	v0 := NewValue(1, 1.0)
	v1 := NewValue(2, 2.0)
	v2 := NewValue(3, 3.0)
	values := Values{v0, v1, v2}
	valuesSize := uint64(v0.Size() + v1.Size() + v2.Size())

	c := NewCache(30 * valuesSize)

	if err := c.WriteMulti(map[string][]Value{"foo": values, "bar": values}); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}
	if n := c.Size(); n != 2*valuesSize+6 {
		t.Fatalf("cache size incorrect after 2 writes, exp %d, got %d", 2*valuesSize, n)
	}

	if exp, keys := [][]byte{[]byte("bar"), []byte("foo")}, c.Keys(); !reflect.DeepEqual(keys, exp) {
		t.Fatalf("cache keys incorrect after 2 writes, exp %v, got %v", exp, keys)
	}
}

// Tests that the cache stats and size are correctly maintained during writes.
func TestCache_WriteMulti_Stats(t *testing.T) {
	vf := NewValue(1, 1.0)
	vi := NewValue(1, int64(1))
	c := NewCache(60)

	// Fail one of the values in the write.
	if err := c.WriteMulti(map[string][]Value{"foo": {vf}}); err != nil {
		t.Fatalf("expected no error. got %v", err)
	}
	if err := c.WriteMulti(map[string][]Value{"foo": {vi}, "bar": {vf}}); err == nil {
		t.Fatal("got no error")
	}

	// Not enough room in the cache.
	if err := c.WriteMulti(map[string][]Value{"foo": {vf, vf}}); err == nil {
		t.Fatal("got no error")
	}

	// Cache size decreased correctly.
	if got, exp := c.Size(), uint64(3+3*8+3+8); got != exp {
		t.Fatalf("got %v, expected %v", got, exp)
	}

	// Write stats updated
	if got, exp := atomic.LoadUint64(&c.tracker.writesDropped), uint64(1); got != exp {
		t.Fatalf("got %v, expected %v", got, exp)
	} else if got, exp := atomic.LoadUint64(&c.tracker.writesErr), uint64(2); got != exp {
		t.Fatalf("got %v, expected %v", got, exp)
	}
}

func TestCache_CacheWriteMulti_TypeConflict(t *testing.T) {
	v0 := NewValue(1, 1.0)
	v1 := NewValue(2, 2.0)
	v2 := NewValue(3, int64(3))
	values := Values{v0, v1, v2}
	valuesSize := uint64(v0.Size() + v1.Size() + v2.Size())

	c := NewCache(3 * valuesSize)

	if err := c.WriteMulti(map[string][]Value{"foo": values[:1], "bar": values[1:]}); err == nil {
		t.Fatalf(" expected field type conflict")
	}

	if exp, got := uint64(v0.Size())+3, c.Size(); exp != got {
		t.Fatalf("cache size incorrect after 2 writes, exp %d, got %d", exp, got)
	}

	if exp, keys := [][]byte{[]byte("foo")}, c.Keys(); !reflect.DeepEqual(keys, exp) {
		t.Fatalf("cache keys incorrect after 2 writes, exp %v, got %v", exp, keys)
	}
}

func TestCache_Cache_DeleteBucketRange(t *testing.T) {
	v0 := NewValue(1, 1.0)
	v1 := NewValue(2, 2.0)
	v2 := NewValue(3, 3.0)
	values := Values{v0, v1, v2}
	valuesSize := uint64(v0.Size() + v1.Size() + v2.Size())

	c := NewCache(30 * valuesSize)

	if err := c.WriteMulti(map[string][]Value{"foo": values, "bar": values}); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}
	if n := c.Size(); n != 2*valuesSize+6 {
		t.Fatalf("cache size incorrect after 2 writes, exp %d, got %d", 2*valuesSize, n)
	}

	if exp, keys := [][]byte{[]byte("bar"), []byte("foo")}, c.Keys(); !reflect.DeepEqual(keys, exp) {
		t.Fatalf("cache keys incorrect after 2 writes, exp %v, got %v", exp, keys)
	}

	c.DeleteBucketRange(context.Background(), []byte("bar"), 2, math.MaxInt64, nil)

	if exp, keys := [][]byte{[]byte("bar"), []byte("foo")}, c.Keys(); !reflect.DeepEqual(keys, exp) {
		t.Fatalf("cache keys incorrect after delete, exp %v, got %v", exp, keys)
	}

	if got, exp := c.Size(), valuesSize+uint64(v0.Size())+6; exp != got {
		t.Fatalf("cache size incorrect after delete, exp %d, got %d", exp, got)
	}

	if got, exp := len(c.Values([]byte("bar"))), 1; got != exp {
		t.Fatalf("cache values mismatch: got %v, exp %v", got, exp)
	}

	if got, exp := len(c.Values([]byte("foo"))), 3; got != exp {
		t.Fatalf("cache values mismatch: got %v, exp %v", got, exp)
	}
}

func TestCache_DeleteBucketRange_NoValues(t *testing.T) {
	v0 := NewValue(1, 1.0)
	v1 := NewValue(2, 2.0)
	v2 := NewValue(3, 3.0)
	values := Values{v0, v1, v2}
	valuesSize := uint64(v0.Size() + v1.Size() + v2.Size())

	c := NewCache(3 * valuesSize)

	if err := c.WriteMulti(map[string][]Value{"foo": values}); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}
	if n := c.Size(); n != valuesSize+3 {
		t.Fatalf("cache size incorrect after 2 writes, exp %d, got %d", 2*valuesSize, n)
	}

	if exp, keys := [][]byte{[]byte("foo")}, c.Keys(); !reflect.DeepEqual(keys, exp) {
		t.Fatalf("cache keys incorrect after 2 writes, exp %v, got %v", exp, keys)
	}

	c.DeleteBucketRange(context.Background(), []byte("foo"), math.MinInt64, math.MaxInt64, nil)

	if exp, keys := 0, len(c.Keys()); !reflect.DeepEqual(keys, exp) {
		t.Fatalf("cache keys incorrect after 2 writes, exp %v, got %v", exp, keys)
	}

	if got, exp := c.Size(), uint64(0); exp != got {
		t.Fatalf("cache size incorrect after 2 writes, exp %d, got %d", exp, got)
	}

	if got, exp := len(c.Values([]byte("foo"))), 0; got != exp {
		t.Fatalf("cache values mismatch: got %v, exp %v", got, exp)
	}
}

func TestCache_DeleteBucketRange_NotSorted(t *testing.T) {
	v0 := NewValue(1, 1.0)
	v1 := NewValue(3, 3.0)
	v2 := NewValue(2, 2.0)
	values := Values{v0, v1, v2}
	valuesSize := uint64(v0.Size() + v1.Size() + v2.Size())

	c := NewCache(3 * valuesSize)

	if err := c.WriteMulti(map[string][]Value{"foo": values}); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}
	if n := c.Size(); n != valuesSize+3 {
		t.Fatalf("cache size incorrect after 2 writes, exp %d, got %d", 2*valuesSize, n)
	}

	if exp, keys := [][]byte{[]byte("foo")}, c.Keys(); !reflect.DeepEqual(keys, exp) {
		t.Fatalf("cache keys incorrect after 2 writes, exp %v, got %v", exp, keys)
	}

	c.DeleteBucketRange(context.Background(), []byte("foo"), 1, 3, nil)

	if exp, keys := 0, len(c.Keys()); !reflect.DeepEqual(keys, exp) {
		t.Fatalf("cache keys incorrect after delete, exp %v, got %v", exp, keys)
	}

	if got, exp := c.Size(), uint64(0); exp != got {
		t.Fatalf("cache size incorrect after delete, exp %d, got %d", exp, got)
	}

	if got, exp := len(c.Values([]byte("foo"))), 0; got != exp {
		t.Fatalf("cache values mismatch: got %v, exp %v", got, exp)
	}
}

func TestCache_DeleteBucketRange_NonExistent(t *testing.T) {
	c := NewCache(1024)

	c.DeleteBucketRange(context.Background(), []byte("bar"), math.MinInt64, math.MaxInt64, nil)

	if got, exp := c.Size(), uint64(0); exp != got {
		t.Fatalf("cache size incorrect exp %d, got %d", exp, got)
	}
}

type stringPredicate string

func (s stringPredicate) Matches(k []byte) bool    { return string(s) == string(k) }
func (s stringPredicate) Marshal() ([]byte, error) { return nil, errors.New("unused") }

func TestCache_Cache_DeleteBucketRange_WithPredicate(t *testing.T) {
	v0 := NewValue(1, 1.0)
	v1 := NewValue(2, 2.0)
	v2 := NewValue(3, 3.0)
	values := Values{v0, v1, v2}
	valuesSize := uint64(v0.Size() + v1.Size() + v2.Size())

	c := NewCache(30 * valuesSize)

	if err := c.WriteMulti(map[string][]Value{"foo": values, "fee": values}); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}
	if n := c.Size(); n != 2*valuesSize+6 {
		t.Fatalf("cache size incorrect after 2 writes, exp %d, got %d", 2*valuesSize, n)
	}

	if exp, keys := [][]byte{[]byte("fee"), []byte("foo")}, c.Keys(); !reflect.DeepEqual(keys, exp) {
		t.Fatalf("cache keys incorrect after 2 writes, exp %v, got %v", exp, keys)
	}

	c.DeleteBucketRange(context.Background(), []byte("f"), 2, math.MaxInt64, stringPredicate("fee"))

	if exp, keys := [][]byte{[]byte("fee"), []byte("foo")}, c.Keys(); !reflect.DeepEqual(keys, exp) {
		t.Fatalf("cache keys incorrect after delete, exp %v, got %v", exp, keys)
	}

	if got, exp := c.Size(), valuesSize+uint64(v0.Size())+6; exp != got {
		t.Fatalf("cache size incorrect after delete, exp %d, got %d", exp, got)
	}

	if got, exp := len(c.Values([]byte("fee"))), 1; got != exp {
		t.Fatalf("cache values mismatch: got %v, exp %v", got, exp)
	}

	if got, exp := len(c.Values([]byte("foo"))), 3; got != exp {
		t.Fatalf("cache values mismatch: got %v, exp %v", got, exp)
	}
}

// This tests writing two batches to the same series.  The first batch
// is sorted.  The second batch is also sorted but contains duplicates.
func TestCache_CacheWriteMulti_Duplicates(t *testing.T) {
	v0 := NewValue(2, 1.0)
	v1 := NewValue(3, 1.0)
	values0 := Values{v0, v1}

	v3 := NewValue(4, 2.0)
	v4 := NewValue(5, 3.0)
	v5 := NewValue(5, 3.0)
	values1 := Values{v3, v4, v5}

	c := NewCache(0)

	if err := c.WriteMulti(map[string][]Value{"foo": values0}); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}

	if err := c.WriteMulti(map[string][]Value{"foo": values1}); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}

	if exp, keys := [][]byte{[]byte("foo")}, c.Keys(); !reflect.DeepEqual(keys, exp) {
		t.Fatalf("cache keys incorrect after 2 writes, exp %v, got %v", exp, keys)
	}

	expAscValues := Values{v0, v1, v3, v5}
	if exp, got := len(expAscValues), len(c.Values([]byte("foo"))); exp != got {
		t.Fatalf("value count mismatch: exp: %v, got %v", exp, got)
	}
	if deduped := c.Values([]byte("foo")); !reflect.DeepEqual(expAscValues, deduped) {
		t.Fatalf("deduped ascending values for foo incorrect, exp: %v, got %v", expAscValues, deduped)
	}
}

func TestCache_CacheValues(t *testing.T) {
	v0 := NewValue(1, 0.0)
	v1 := NewValue(2, 2.0)
	v2 := NewValue(3, 3.0)
	v3 := NewValue(1, 1.0)
	v4 := NewValue(4, 4.0)

	c := NewCache(512)
	if deduped := c.Values([]byte("no such key")); deduped != nil {
		t.Fatalf("Values returned for no such key")
	}

	if err := c.Write([]byte("foo"), Values{v0, v1, v2, v3}); err != nil {
		t.Fatalf("failed to write 3 values, key foo to cache: %s", err.Error())
	}
	if err := c.Write([]byte("foo"), Values{v4}); err != nil {
		t.Fatalf("failed to write 1 value, key foo to cache: %s", err.Error())
	}

	expAscValues := Values{v3, v1, v2, v4}
	if deduped := c.Values([]byte("foo")); !reflect.DeepEqual(expAscValues, deduped) {
		t.Fatalf("deduped ascending values for foo incorrect, exp: %v, got %v", expAscValues, deduped)
	}
}

func TestCache_CacheSnapshot(t *testing.T) {
	v0 := NewValue(2, 0.0)
	v1 := NewValue(3, 2.0)
	v2 := NewValue(4, 3.0)
	v3 := NewValue(5, 4.0)
	v4 := NewValue(6, 5.0)
	v5 := NewValue(1, 5.0)
	v6 := NewValue(7, 5.0)
	v7 := NewValue(2, 5.0)

	c := NewCache(512)
	if err := c.Write([]byte("foo"), Values{v0, v1, v2, v3}); err != nil {
		t.Fatalf("failed to write 3 values, key foo to cache: %s", err.Error())
	}

	// Grab snapshot, and ensure it's as expected.
	snapshot, err := c.Snapshot()
	if err != nil {
		t.Fatalf("failed to snapshot cache: %v", err)
	}

	expValues := Values{v0, v1, v2, v3}
	if deduped := snapshot.values([]byte("foo")); !reflect.DeepEqual(expValues, deduped) {
		t.Fatalf("snapshotted values for foo incorrect, exp: %v, got %v", expValues, deduped)
	}

	// Ensure cache is still as expected.
	if deduped := c.Values([]byte("foo")); !reflect.DeepEqual(expValues, deduped) {
		t.Fatalf("post-snapshot values for foo incorrect, exp: %v, got %v", expValues, deduped)
	}

	// Write a new value to the cache.
	if err := c.Write([]byte("foo"), Values{v4}); err != nil {
		t.Fatalf("failed to write post-snap value, key foo to cache: %s", err.Error())
	}
	expValues = Values{v0, v1, v2, v3, v4}
	if deduped := c.Values([]byte("foo")); !reflect.DeepEqual(expValues, deduped) {
		t.Fatalf("post-snapshot write values for foo incorrect, exp: %v, got %v", expValues, deduped)
	}

	// Write a new, out-of-order, value to the cache.
	if err := c.Write([]byte("foo"), Values{v5}); err != nil {
		t.Fatalf("failed to write post-snap value, key foo to cache: %s", err.Error())
	}
	expValues = Values{v5, v0, v1, v2, v3, v4}
	if deduped := c.Values([]byte("foo")); !reflect.DeepEqual(expValues, deduped) {
		t.Fatalf("post-snapshot out-of-order write values for foo incorrect, exp: %v, got %v", expValues, deduped)
	}

	// Clear snapshot, ensuring non-snapshot data untouched.
	c.ClearSnapshot(true)

	expValues = Values{v5, v4}
	if deduped := c.Values([]byte("foo")); !reflect.DeepEqual(expValues, deduped) {
		t.Fatalf("post-clear values for foo incorrect, exp: %v, got %v", expValues, deduped)
	}

	// Create another snapshot
	_, err = c.Snapshot()
	if err != nil {
		t.Fatalf("failed to snapshot cache: %v", err)
	}

	if err := c.Write([]byte("foo"), Values{v4, v5}); err != nil {
		t.Fatalf("failed to write post-snap value, key foo to cache: %s", err.Error())
	}

	c.ClearSnapshot(true)

	_, err = c.Snapshot()
	if err != nil {
		t.Fatalf("failed to snapshot cache: %v", err)
	}

	if err := c.Write([]byte("foo"), Values{v6, v7}); err != nil {
		t.Fatalf("failed to write post-snap value, key foo to cache: %s", err.Error())
	}

	expValues = Values{v5, v7, v4, v6}
	if deduped := c.Values([]byte("foo")); !reflect.DeepEqual(expValues, deduped) {
		t.Fatalf("post-snapshot out-of-order write values for foo incorrect, exp: %v, got %v", expValues, deduped)
	}
}

// Tests that Snapshot updates statistics correctly.
func TestCache_Snapshot_Stats(t *testing.T) {
	limit := uint64(16)
	c := NewCache(limit)

	values := map[string][]Value{"foo": {NewValue(1, 1.0)}}
	if err := c.WriteMulti(values); err != nil {
		t.Fatal(err)
	}

	if got, exp := atomic.LoadUint64(&c.tracker.memSizeBytes), uint64(16)+3; got != exp {
		t.Fatalf("got %v, expected %v", got, exp)
	}

	_, err := c.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	// Store size should have been reset.
	if got, exp := c.Size(), uint64(16)+3; got != exp {
		t.Fatalf("got %v, expected %v", got, exp)
	}

	// Cached bytes should have been increased.
	if got, exp := atomic.LoadUint64(&c.tracker.snapshottedBytes), uint64(16)+3; got != exp {
		t.Fatalf("got %v, expected %v", got, exp)
	}

	if got, exp := atomic.LoadUint64(&c.tracker.memSizeBytes), uint64(16)+3; got != exp {
		t.Fatalf("got %v, expected %v", got, exp)
	}
}

func TestCache_CacheEmptySnapshot(t *testing.T) {
	c := NewCache(512)

	// Grab snapshot, and ensure it's as expected.
	snapshot, err := c.Snapshot()
	if err != nil {
		t.Fatalf("failed to snapshot cache: %v", err)
	}
	if deduped := snapshot.values([]byte("foo")); !reflect.DeepEqual(Values(nil), deduped) {
		t.Fatalf("snapshotted values for foo incorrect, exp: %v, got %v", nil, deduped)
	}

	// Ensure cache is still as expected.
	if deduped := c.Values([]byte("foo")); !reflect.DeepEqual(Values(nil), deduped) {
		t.Fatalf("post-snapshotted values for foo incorrect, exp: %v, got %v", Values(nil), deduped)
	}

	// Clear snapshot.
	c.ClearSnapshot(true)
	if deduped := c.Values([]byte("foo")); !reflect.DeepEqual(Values(nil), deduped) {
		t.Fatalf("post-snapshot-clear values for foo incorrect, exp: %v, got %v", Values(nil), deduped)
	}
}

func TestCache_CacheWriteMemoryExceeded(t *testing.T) {
	v0 := NewValue(1, 1.0)
	v1 := NewValue(2, 2.0)

	c := NewCache(uint64(v1.Size()))

	if err := c.Write([]byte("foo"), Values{v0}); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}
	if exp, keys := [][]byte{[]byte("foo")}, c.Keys(); !reflect.DeepEqual(keys, exp) {
		t.Fatalf("cache keys incorrect after writes, exp %v, got %v", exp, keys)
	}
	if err := c.Write([]byte("bar"), Values{v1}); err == nil || !strings.Contains(err.Error(), "cache-max-memory-size") {
		t.Fatalf("wrong error writing key bar to cache: %v", err)
	}

	// Grab snapshot, write should still fail since we're still using the memory.
	_, err := c.Snapshot()
	if err != nil {
		t.Fatalf("failed to snapshot cache: %v", err)
	}
	if err := c.Write([]byte("bar"), Values{v1}); err == nil || !strings.Contains(err.Error(), "cache-max-memory-size") {
		t.Fatalf("wrong error writing key bar to cache: %v", err)
	}

	// Clear the snapshot and the write should now succeed.
	c.ClearSnapshot(true)
	if err := c.Write([]byte("bar"), Values{v1}); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}
	expAscValues := Values{v1}
	if deduped := c.Values([]byte("bar")); !reflect.DeepEqual(expAscValues, deduped) {
		t.Fatalf("deduped ascending values for bar incorrect, exp: %v, got %v", expAscValues, deduped)
	}
}

func TestCache_Deduplicate_Concurrent(t *testing.T) {
	if testing.Short() || os.Getenv("GORACE") != "" || os.Getenv("APPVEYOR") != "" {
		t.Skip("Skipping test in short, race, appveyor mode.")
	}

	values := make(map[string][]Value)

	for i := 0; i < 1000; i++ {
		for j := 0; j < 100; j++ {
			values[fmt.Sprintf("cpu%d", i)] = []Value{NewValue(int64(i+j)+int64(rand.Intn(10)), float64(i))}
		}
	}

	wg := sync.WaitGroup{}
	c := NewCache(1000000)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			c.WriteMulti(values)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			c.Deduplicate()
		}
	}()

	wg.Wait()
}

// Ensure the CacheLoader can correctly load from a single segment, even if it's corrupted.
func TestCacheLoader_LoadSingle(t *testing.T) {
	// Create a WAL segment.
	dir := mustTempDir()
	defer os.RemoveAll(dir)
	f := mustTempFile(dir)
	w := wal.NewWALSegmentWriter(f)

	p1 := NewValue(1, 1.1)
	p2 := NewValue(1, int64(1))
	p3 := NewValue(1, true)

	values := map[string][]Value{
		"foo": {p1},
		"bar": {p2},
		"baz": {p3},
	}

	entry := &wal.WriteWALEntry{
		Values: values,
	}

	if err := w.Write(mustMarshalEntry(entry)); err != nil {
		t.Fatal("write points", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("flush error: %v", err)
	}

	// Load the cache using the segment.
	cache := NewCache(1024)
	loader := NewCacheLoader([]string{f.Name()})
	if err := loader.Load(cache); err != nil {
		t.Fatalf("failed to load cache: %s", err.Error())
	}

	// Check the cache.
	if values := cache.Values([]byte("foo")); !reflect.DeepEqual(values, Values{p1}) {
		t.Fatalf("cache key foo not as expected, got %v, exp %v", values, Values{p1})
	}
	if values := cache.Values([]byte("bar")); !reflect.DeepEqual(values, Values{p2}) {
		t.Fatalf("cache key foo not as expected, got %v, exp %v", values, Values{p2})
	}
	if values := cache.Values([]byte("baz")); !reflect.DeepEqual(values, Values{p3}) {
		t.Fatalf("cache key foo not as expected, got %v, exp %v", values, Values{p3})
	}

	// Corrupt the WAL segment.
	if _, err := f.Write([]byte{1, 4, 0, 0, 0}); err != nil {
		t.Fatalf("corrupt WAL segment: %s", err.Error())
	}

	// Reload the cache using the segment.
	cache = NewCache(1024)
	loader = NewCacheLoader([]string{f.Name()})
	if err := loader.Load(cache); err != nil {
		t.Fatalf("failed to load cache: %s", err.Error())
	}

	// Check the cache.
	if values := cache.Values([]byte("foo")); !reflect.DeepEqual(values, Values{p1}) {
		t.Fatalf("cache key foo not as expected, got %v, exp %v", values, Values{p1})
	}
	if values := cache.Values([]byte("bar")); !reflect.DeepEqual(values, Values{p2}) {
		t.Fatalf("cache key bar not as expected, got %v, exp %v", values, Values{p2})
	}
	if values := cache.Values([]byte("baz")); !reflect.DeepEqual(values, Values{p3}) {
		t.Fatalf("cache key baz not as expected, got %v, exp %v", values, Values{p3})
	}
}

// Ensure the CacheLoader can correctly load from two segments, even if one is corrupted.
func TestCacheLoader_LoadDouble(t *testing.T) {
	// Create a WAL segment.
	dir := mustTempDir()
	defer os.RemoveAll(dir)
	f1, f2 := mustTempFile(dir), mustTempFile(dir)
	w1, w2 := wal.NewWALSegmentWriter(f1), wal.NewWALSegmentWriter(f2)

	p1 := NewValue(1, 1.1)
	p2 := NewValue(1, int64(1))
	p3 := NewValue(1, true)
	p4 := NewValue(1, "string")

	// Write first and second segment.

	segmentWrite := func(w *wal.WALSegmentWriter, values map[string][]Value) {
		entry := &wal.WriteWALEntry{
			Values: values,
		}
		if err := w1.Write(mustMarshalEntry(entry)); err != nil {
			t.Fatal("write points", err)
		}
		if err := w1.Flush(); err != nil {
			t.Fatalf("flush error: %v", err)
		}
	}

	values := map[string][]Value{
		"foo": {p1},
		"bar": {p2},
	}
	segmentWrite(w1, values)
	values = map[string][]Value{
		"baz": {p3},
		"qux": {p4},
	}
	segmentWrite(w2, values)

	// Corrupt the first WAL segment.
	if _, err := f1.Write([]byte{1, 4, 0, 0, 0}); err != nil {
		t.Fatalf("corrupt WAL segment: %s", err.Error())
	}

	// Load the cache using the segments.
	cache := NewCache(1024)
	loader := NewCacheLoader([]string{f1.Name(), f2.Name()})
	if err := loader.Load(cache); err != nil {
		t.Fatalf("failed to load cache: %s", err.Error())
	}

	// Check the cache.
	if values := cache.Values([]byte("foo")); !reflect.DeepEqual(values, Values{p1}) {
		t.Fatalf("cache key foo not as expected, got %v, exp %v", values, Values{p1})
	}
	if values := cache.Values([]byte("bar")); !reflect.DeepEqual(values, Values{p2}) {
		t.Fatalf("cache key bar not as expected, got %v, exp %v", values, Values{p2})
	}
	if values := cache.Values([]byte("baz")); !reflect.DeepEqual(values, Values{p3}) {
		t.Fatalf("cache key baz not as expected, got %v, exp %v", values, Values{p3})
	}
	if values := cache.Values([]byte("qux")); !reflect.DeepEqual(values, Values{p4}) {
		t.Fatalf("cache key qux not as expected, got %v, exp %v", values, Values{p4})
	}
}

func TestCache_Split(t *testing.T) {
	v0 := NewValue(1, 1.0)
	v1 := NewValue(2, 2.0)
	v2 := NewValue(3, 3.0)
	values := Values{v0, v1, v2}
	valuesSize := uint64(v0.Size() + v1.Size() + v2.Size())

	c := NewCache(0)

	if err := c.Write([]byte("foo"), values); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}
	if err := c.Write([]byte("bar"), values); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}

	if err := c.Write([]byte("baz"), values); err != nil {
		t.Fatalf("failed to write key foo to cache: %s", err.Error())
	}

	if n := c.Size(); n != 3*valuesSize+9 {
		t.Fatalf("cache size incorrect after 3 writes, exp %d, got %d", 3*valuesSize*9, n)
	}

	splits := c.Split(3)
	keys := make(map[string]int)
	for _, s := range splits {
		for _, k := range s.Keys() {
			keys[string(k)] = s.Values(k).Size()
		}
	}

	for _, key := range []string{"foo", "bar", "baz"} {
		if _, ok := keys[key]; !ok {
			t.Fatalf("missing key, exp %s, got %v", key, nil)
		}
	}
}

func mustTempDir() string {
	dir, err := ioutil.TempDir("", "tsm1-test")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp dir: %v", err))
	}
	return dir
}

func mustTempFile(dir string) *os.File {
	f, err := ioutil.TempFile(dir, "tsm1test")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp file: %v", err))
	}
	return f
}

func mustMarshalEntry(entry wal.WALEntry) (wal.WalEntryType, []byte) {
	bytes := make([]byte, 1024<<2)

	b, err := entry.Encode(bytes)
	if err != nil {
		panic(fmt.Sprintf("error encoding: %v", err))
	}

	return entry.Type(), snappy.Encode(b, b)
}

var fvSize = uint64(NewValue(1, float64(1)).Size())

func BenchmarkCacheFloatEntries(b *testing.B) {
	cache := NewCache(uint64(b.N)*fvSize + 4)
	vals := make([][]Value, b.N)
	for i := 0; i < b.N; i++ {
		vals[i] = []Value{NewValue(1, float64(i))}
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := cache.Write([]byte("test"), vals[i]); err != nil {
			b.Fatal("err:", err, "i:", i, "N:", b.N)
		}
	}
}

type points struct {
	key  []byte
	vals []Value
}

func BenchmarkCacheParallelFloatEntries(b *testing.B) {
	c := b.N * runtime.GOMAXPROCS(0)
	cache := NewCache(uint64(c)*fvSize*10 + 20*5)
	vals := make([]points, c)
	for i := 0; i < c; i++ {
		v := make([]Value, 10)
		for j := 0; j < 10; j++ {
			v[j] = NewValue(1, float64(i+j))
		}
		vals[i] = points{key: []byte(fmt.Sprintf("cpu%v", rand.Intn(20))), vals: v}
	}
	i := int32(-1)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			j := atomic.AddInt32(&i, 1)
			v := vals[j]
			if err := cache.Write(v.key, v.vals); err != nil {
				b.Fatal("err:", err, "j:", j, "N:", b.N)
			}
		}
	})
}

func BenchmarkEntry_add(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			b.StopTimer()
			values := make([]Value, 10)
			for i := 0; i < 10; i++ {
				values[i] = NewValue(int64(i+1), float64(i))
			}

			otherValues := make([]Value, 10)
			for i := 0; i < 10; i++ {
				otherValues[i] = NewValue(1, float64(i))
			}

			entry, err := newEntryValues(values)
			if err != nil {
				b.Fatal(err)
			}

			b.StartTimer()
			if err := entry.add(otherValues); err != nil {
				b.Fatal(err)
			}
		}
	})
}
