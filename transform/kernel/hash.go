package kernel

import (
	"context"
	"crypto/md5"  //nolint:gosec // a digest for masking and keys, not for security
	"crypto/sha1" //nolint:gosec // a digest for masking and keys, not for security
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"strconv"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/compute"
)

// Digests take one or more columns and give back the lowercase hex digest of
// their text joined with "-". A utf8 value, which is also how a uuid travels,
// is its bytes; an integer is its decimal text, which is what a warehouse
// does when the column is cast before hashing. Other types would need a text
// form chosen for them, and no two systems choose the same one, so they are
// not accepted: cast first and the text form is yours.
//
// A null column contributes nothing between the separators, so a key built
// from several columns survives a missing value; a row where every column is
// null comes out null, so a masked column keeps its nulls.

// Md5 is the MD5 digest of one or more columns.
func Md5(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return digest(ctx, args, md5.New()) //nolint:gosec // masking and keys, not security
}

// Sha1 is the SHA-1 digest of one or more columns.
func Sha1(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return digest(ctx, args, sha1.New()) //nolint:gosec // masking and keys, not security
}

// Sha256 is the SHA-256 digest of one or more columns.
func Sha256(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return digest(ctx, args, sha256.New())
}

// Sha512 is the SHA-512 digest of one or more columns.
func Sha512(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return digest(ctx, args, sha512.New())
}

// digest runs one pass over the batch, joining each row's values into a
// scratch buffer and hashing it. The hash, its sum, and both buffers are
// reused across rows.
func digest(ctx context.Context, args []compute.Datum, h hash.Hash) (compute.Datum, error) {
	n := -1
	cols := make([]arrow.Array, len(args))
	texts := make([]func(i int, dst []byte) []byte, len(args))
	defer func() {
		for _, c := range cols {
			if c != nil {
				c.Release()
			}
		}
	}()
	for i, a := range args {
		ad, ok := a.(*compute.ArrayDatum)
		if !ok {
			return nil, fmt.Errorf("expected a column, got %s", a)
		}
		cols[i] = array.MakeFromData(ad.Value)
		text, err := textValue(cols[i])
		if err != nil {
			return nil, err
		}
		texts[i], n = text, cols[i].Len()
	}
	if n < 0 {
		return nil, fmt.Errorf("expected at least one column")
	}

	b := array.NewStringBuilder(compute.GetAllocator(ctx))
	defer b.Release()
	b.Reserve(n)
	sum := make([]byte, 0, h.Size())
	var joined, hexed []byte
	for row := range n {
		joined = joined[:0]
		present := false
		for i, c := range cols {
			if i > 0 {
				joined = append(joined, '-')
			}
			if c.IsNull(row) {
				continue
			}
			joined, present = texts[i](row, joined), true
		}
		if !present {
			b.AppendNull()
			continue
		}
		h.Reset()
		h.Write(joined)
		sum = h.Sum(sum[:0])
		hexed = hex.AppendEncode(hexed[:0], sum)
		b.BinaryBuilder.Append(hexed)
	}
	out := b.NewArray()
	defer out.Release()
	return compute.NewDatum(out), nil
}

// textValue returns a function that appends row i's text to dst: a utf8
// value as its bytes, an integer as decimal.
func textValue(a arrow.Array) (func(i int, dst []byte) []byte, error) {
	switch v := a.(type) {
	case *array.String:
		return func(i int, dst []byte) []byte { return append(dst, v.Value(i)...) }, nil
	case *array.Int16:
		return func(i int, dst []byte) []byte { return strconv.AppendInt(dst, int64(v.Value(i)), 10) }, nil
	case *array.Int32:
		return func(i int, dst []byte) []byte { return strconv.AppendInt(dst, int64(v.Value(i)), 10) }, nil
	case *array.Int64:
		return func(i int, dst []byte) []byte { return strconv.AppendInt(dst, v.Value(i), 10) }, nil
	}
	return nil, fmt.Errorf("expected a utf8 or integer array, got %s", a.DataType())
}
