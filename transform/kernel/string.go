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
	"github.com/apache/arrow-go/v18/arrow/scalar"
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

// Upper uppercases every value of a utf8 column.
func Upper(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return mapBytes(ctx, args[0], upperBytes)
}

// upperBytes mirrors lowerBytes.
func upperBytes(dst, s []byte) []byte {
	for _, c := range s {
		if c >= utf8.RuneSelf {
			return append(dst[:0], strings.ToUpper(string(s))...)
		}
		if 'a' <= c && c <= 'z' {
			c -= 'a' - 'A'
		}
		dst = append(dst, c)
	}
	return dst
}

// Length counts the characters, not bytes, of every value of a utf8 column.
func Length(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	src, err := stringArray(args[0])
	if err != nil {
		return nil, err
	}
	defer src.Release()
	b := array.NewInt64Builder(compute.GetAllocator(ctx))
	defer b.Release()
	b.Reserve(src.Len())
	for i := range src.Len() {
		if src.IsNull(i) {
			b.AppendNull()
			continue
		}
		b.UnsafeAppend(int64(utf8.RuneCountInString(src.Value(i))))
	}
	out := b.NewArray()
	defer out.Release()
	return compute.NewDatum(out), nil
}

// Replace swaps every occurrence of find with with in every value of a utf8
// column. Both are literals.
func Replace(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	find, err := stringScalar(args[1])
	if err != nil {
		return nil, err
	}
	with, err := stringScalar(args[2])
	if err != nil {
		return nil, err
	}
	if len(find) == 0 {
		return mapBytes(ctx, args[0], func(_, s []byte) []byte { return s })
	}
	return mapBytes(ctx, args[0], func(dst, s []byte) []byte {
		for {
			i := bytes.Index(s, find)
			if i < 0 {
				return append(dst, s...)
			}
			dst = append(append(dst, s[:i]...), with...)
			s = s[i+len(find):]
		}
	})
}

// Substring slices every value of a utf8 column by character position,
// counting from 1 as SQL does. Without a length it runs to the end.
func Substring(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	start, err := int64Scalar(args[1])
	if err != nil {
		return nil, err
	}
	length := int64(-1)
	if len(args) > 2 {
		if length, err = int64Scalar(args[2]); err != nil {
			return nil, err
		}
	}
	return mapBytes(ctx, args[0], func(_, s []byte) []byte {
		from := runeOffset(s, start-1)
		if length < 0 {
			return s[from:]
		}
		return s[from : from+runeOffset(s[from:], length)]
	})
}

// runeOffset returns the byte offset of the nth character of s, or len(s)
// when s is shorter.
func runeOffset(s []byte, n int64) int {
	off := 0
	for ; n > 0 && off < len(s); n-- {
		_, size := utf8.DecodeRune(s[off:])
		off += size
	}
	return off
}

// Concat joins its arguments end to end for every row. Any argument may be a
// column or a literal; a null in any column part gives a null row.
func Concat(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	n := -1
	parts := make([]*array.String, len(args))
	lits := make([][]byte, len(args))
	defer func() {
		for _, p := range parts {
			if p != nil {
				p.Release()
			}
		}
	}()
	for i, a := range args {
		switch d := a.(type) {
		case *compute.ArrayDatum:
			src, err := stringArray(d)
			if err != nil {
				return nil, err
			}
			parts[i], n = src, src.Len()
		case *compute.ScalarDatum:
			s, err := stringScalar(d)
			if err != nil {
				return nil, err
			}
			lits[i] = s
		}
	}
	if n < 0 {
		return nil, fmt.Errorf("concat: expected at least one column")
	}
	b := array.NewStringBuilder(compute.GetAllocator(ctx))
	defer b.Release()
	b.Reserve(n)
	var scratch []byte
	for row := range n {
		scratch = scratch[:0]
		null := false
		for i := range args {
			if p := parts[i]; p != nil {
				if p.IsNull(row) {
					null = true
					break
				}
				scratch = append(scratch, p.Value(row)...)
				continue
			}
			scratch = append(scratch, lits[i]...)
		}
		if null {
			b.AppendNull()
			continue
		}
		b.BinaryBuilder.Append(scratch)
	}
	out := b.NewArray()
	defer out.Release()
	return compute.NewDatum(out), nil
}

// stringArray borrows a datum as a utf8 array the caller releases.
func stringArray(d compute.Datum) (*array.String, error) {
	ad, ok := d.(*compute.ArrayDatum)
	if !ok || ad.Value.DataType().ID() != arrow.STRING {
		return nil, fmt.Errorf("expected utf8 array, got %s", d)
	}
	return array.NewStringData(ad.Value), nil
}

// stringScalar reads a utf8 literal datum.
func stringScalar(d compute.Datum) ([]byte, error) {
	sd, ok := d.(*compute.ScalarDatum)
	if !ok {
		return nil, fmt.Errorf("expected utf8 literal, got %s", d)
	}
	s, ok := sd.Value.(*scalar.String)
	if !ok {
		return nil, fmt.Errorf("expected utf8 literal, got %s", sd.Value.DataType())
	}
	return s.Value.Bytes(), nil
}

// int64Scalar reads an int64 literal datum.
func int64Scalar(d compute.Datum) (int64, error) {
	sd, ok := d.(*compute.ScalarDatum)
	if !ok {
		return 0, fmt.Errorf("expected int64 literal, got %s", d)
	}
	s, ok := sd.Value.(*scalar.Int64)
	if !ok {
		return 0, fmt.Errorf("expected int64 literal, got %s", sd.Value.DataType())
	}
	return s.Value, nil
}
