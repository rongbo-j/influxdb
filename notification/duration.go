package notification

import (
	"bytes"
	"strconv"

	"github.com/influxdata/flux/ast"
	"github.com/influxdata/flux/parser"
)

// Duration is a custom type used for generating flux compatible durations.
type Duration ast.DurationLiteral

// MarshalJSON turns a Duration into a JSON-ified string.
func (d Duration) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('"')
	for _, d := range d.Values {
		b.WriteString(strconv.Itoa(int(d.Magnitude)))
		b.WriteString(d.Unit)
	}
	b.WriteByte('"')

	return b.Bytes(), nil
}

// UnmarshalJSON turns a flux duration literal into a Duration.
func (d *Duration) UnmarshalJSON(b []byte) error {
	dur, err := parser.ParseDuration(string(b[1 : len(b)-1]))
	if err != nil {
		return err
	}

	*d = *(*Duration)(dur)

	return nil
}
