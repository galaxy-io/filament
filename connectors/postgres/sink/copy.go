package postgres

// COPY payload framing. A batch's Arrow columns are rendered straight into the
// COPY wire format: binary when every column has a binary send form for its
// destination type, text otherwise (arrays, enums, domains, and any other type
// the source projected as text ride the text format for the whole table). The
// per-type renderers live in types.go.

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
)

// copier renders rows of one Arrow schema as a COPY payload for a subset of the
// destination table's columns.
type copier struct {
	binary bool
	cols   []copyCol
}

// copyCol is one destination column: its Arrow column and value renderers.
type copyCol struct {
	idx  int
	bin  valueFn // value bytes without the length prefix; nil = no binary form
	text valueFn // COPY-escaped text
}

type valueFn func(dst []byte, col arrow.Array, i int) []byte

var copyCRCTable = crc32.MakeTable(crc32.Castagnoli)

// newCopier prepares renderers for the Arrow columns idx of schema, landing in
// destination columns of the given Postgres types.
func newCopier(schema *arrow.Schema, idx []int, pgTypes []string) *copier {
	c := &copier{binary: true, cols: make([]copyCol, len(idx))}
	for n, i := range idx {
		f := schema.Field(i)
		c.cols[n] = copyCol{idx: i, bin: binaryFor(f, pgTypes[n]), text: textFor(f)}
		if c.cols[n].bin == nil {
			c.binary = false
		}
	}
	return c
}

// sql builds the COPY statement for the copier's format.
func (c *copier) sql(qualified string, idents []string) string {
	s := "COPY " + qualified + " (" + strings.Join(idents, ", ") + ") FROM STDIN"
	if c.binary {
		s += " BINARY"
	}
	return s
}

// encode renders rows [lo, hi) of rows as one complete COPY payload; ordered adds
// a trailing _ord column holding each row's position in the range, for folds that
// must keep batch order.
func (c *copier) encode(rows arrow.RecordBatch, lo, hi int, ordered bool) ([]byte, uint32) {
	dst := make([]byte, 0, len(copyHeader)+(hi-lo)*8*(len(c.cols)+1))
	if c.binary {
		dst = c.encodeBinary(dst, rows, lo, hi, ordered)
	} else {
		dst = c.encodeText(dst, rows, lo, hi, ordered)
	}
	return dst, crc32.Checksum(dst, copyCRCTable)
}

// verifyCopyChecksum rechecks the exact COPY payload immediately before the
// transport write.
func verifyCopyChecksum(payload []byte, expected uint32) error {
	actual := crc32.Checksum(payload, copyCRCTable)
	if actual != expected {
		return fmt.Errorf("COPY serialization CRC divergence: encoded %08x, expected %08x", actual, expected)
	}
	return nil
}

// copyHeader opens a binary COPY stream: signature, flags, extension length.
var copyHeader = []byte("PGCOPY\n\377\r\n\000\x00\x00\x00\x00\x00\x00\x00\x00")

func (c *copier) encodeBinary(dst []byte, rows arrow.RecordBatch, lo, hi int, ord bool) []byte {
	dst = append(dst, copyHeader...)
	cols := rows.Columns()
	ncols := len(c.cols)
	if ord {
		ncols++
	}
	for i := lo; i < hi; i++ {
		dst = binary.BigEndian.AppendUint16(dst, uint16(ncols)) //nolint:gosec // column count
		for _, cc := range c.cols {
			col := cols[cc.idx]
			if col.IsNull(i) {
				dst = append(dst, 0xff, 0xff, 0xff, 0xff)
				continue
			}
			at := len(dst)
			dst = append(dst, 0, 0, 0, 0)
			dst = cc.bin(dst, col, i)
			binary.BigEndian.PutUint32(dst[at:], uint32(len(dst)-at-4)) //nolint:gosec // value length
		}
		if ord {
			dst = binary.BigEndian.AppendUint32(dst, 4)
			dst = binary.BigEndian.AppendUint32(dst, uint32(i-lo)) //nolint:gosec // row position
		}
	}
	return append(dst, 0xff, 0xff) // trailer: -1 column count
}

func (c *copier) encodeText(dst []byte, rows arrow.RecordBatch, lo, hi int, ord bool) []byte {
	cols := rows.Columns()
	for i := lo; i < hi; i++ {
		for n, cc := range c.cols {
			if n > 0 {
				dst = append(dst, '\t')
			}
			col := cols[cc.idx]
			if col.IsNull(i) {
				dst = append(dst, '\\', 'N')
				continue
			}
			dst = cc.text(dst, col, i)
		}
		if ord {
			dst = append(dst, '\t')
			dst = strconv.AppendInt(dst, int64(i-lo), 10)
		}
		dst = append(dst, '\n')
	}
	return dst
}

// copyError wraps a COPY failure with the batch it belongs to.
func copyError(what, resource string, seq uint64, err error) error {
	return fmt.Errorf("postgres sink: %s %s seq %d: %w", what, resource, seq, err)
}
