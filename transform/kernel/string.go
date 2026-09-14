// Package kernel implements the whole-column operations a transform plan
// runs. Each kernel reads its arguments as Arrow datums, leaves them for the
// caller to release, and returns a new datum the caller owns. A null in is a
// null out. The package depends on Arrow alone, so it can leave filament
// without change.
//
// Names follow the yaml grammar: to_date is ToDate, year is Year, lower is
// Lower. Only conversions between types carry the To prefix.
package kernel

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/compute"
)

// Lower lowercases every value of a utf8 column.
func Lower(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return mapBytes(ctx, args[0], lowerBytes)
}

// Trim removes leading and trailing whitespace from every value of a utf8
// column.
func Trim(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return mapBytes(ctx, args[0], func(_, s []byte) []byte { return bytes.TrimSpace(s) })
}

// mapBytes builds a new utf8 array by applying f to every non-null value. f
// gets a reusable scratch buffer and the value's bytes and may return either
// one, so a kernel that rewrites bytes in place allocates nothing per row.
// The output builder is sized to the input up front.
func mapBytes(ctx context.Context, in compute.Datum, f func(scratch, s []byte) []byte) (compute.Datum, error) {
	ad, ok := in.(*compute.ArrayDatum)
	if !ok || ad.Value.DataType().ID() != arrow.STRING {
		return nil, fmt.Errorf("expected utf8 array, got %s", in)
	}
	src := array.NewStringData(ad.Value)
	defer src.Release()
	data, offs := src.ValueBytes(), src.ValueOffsets()

	b := array.NewStringBuilder(compute.GetAllocator(ctx))
	defer b.Release()
	b.Reserve(src.Len())
	b.ReserveData(len(data))

	var scratch []byte
	for i := range src.Len() {
		if src.IsNull(i) {
			b.AppendNull()
			continue
		}
		scratch = f(scratch[:0], data[offs[i]-offs[0]:offs[i+1]-offs[0]])
		b.BinaryBuilder.Append(scratch)
	}
	out := b.NewArray()
	defer out.Release()
	return compute.NewDatum(out), nil
}

// lowerBytes appends the lowercase of s to dst, byte by byte for ASCII. The
// first non-ASCII byte hands the whole value to the Unicode-aware path.
func lowerBytes(dst, s []byte) []byte {
	for _, c := range s {
		if c >= utf8.RuneSelf {
			return append(dst[:0], strings.ToLower(string(s))...)
		}
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		dst = append(dst, c)
	}
	return dst
}
