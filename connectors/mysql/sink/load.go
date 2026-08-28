package mysql

// LOAD DATA payload framing. A batch's Arrow columns are rendered as
// tab-separated lines and streamed to the server through the driver's
// LOAD DATA LOCAL INFILE reader handler.

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"io"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/go-sql-driver/mysql"
)

// loader renders rows of one Arrow schema as a LOAD DATA payload for a subset of
// the destination table's columns.
type loader struct {
	cols []loadCol
}

type loadCol struct {
	idx  int
	text valueFn
}

var loadCRCTable = crc32.MakeTable(crc32.Castagnoli)

// newLoader prepares renderers for the Arrow columns idx of schema.
func newLoader(schema *arrow.Schema, idx []int) *loader {
	l := &loader{cols: make([]loadCol, len(idx))}
	for n, i := range idx {
		l.cols[n] = loadCol{idx: i, text: textFor(schema.Field(i))}
	}
	return l
}

// encode renders rows [lo, hi) as one payload.
func (l *loader) encode(rows arrow.RecordBatch, lo, hi int) ([]byte, uint32) {
	dst := make([]byte, 0, (hi-lo)*8*len(l.cols))
	cols := rows.Columns()
	for i := lo; i < hi; i++ {
		for n, lc := range l.cols {
			if n > 0 {
				dst = append(dst, '\t')
			}
			col := cols[lc.idx]
			if col.IsNull(i) {
				dst = append(dst, '\\', 'N')
				continue
			}
			dst = lc.text(dst, col, i)
		}
		dst = append(dst, '\n')
	}
	return dst, crc32.Checksum(dst, loadCRCTable)
}

// verifyLoadChecksum rechecks the exact LOAD DATA payload immediately before
// the transport write.
func verifyLoadChecksum(payload []byte, expected uint32) error {
	actual := crc32.Checksum(payload, loadCRCTable)
	if actual != expected {
		return fmt.Errorf("LOAD DATA serialization CRC divergence: encoded %08x, expected %08x", actual, expected)
	}
	return nil
}

// readerSeq numbers the payload readers, whose handler names are process-global.
var readerSeq atomic.Uint64

// register hands a payload to the driver under a fresh handler name and returns
// the name (for "Reader::<name>") and the deregistration to defer.
func register(payload []byte) (name string, done func()) {
	name = "filament-" + strconv.FormatUint(readerSeq.Add(1), 10)
	mysql.RegisterReaderHandler(name, func() io.Reader { return bytes.NewReader(payload) })
	return name, func() { mysql.DeregisterReaderHandler(name) }
}

// loadSQL builds the LOAD DATA statement for a payload: local reader, binary
// character set (bytes pass through as rendered), tab/newline framing, backslash
// escapes. replace makes duplicate keys overwrite (the upsert idiom). A JSON
// column cannot take a binary string, so json marks the columns that load
// through a variable converted to utf8mb4.
func loadSQL(reader, qualified string, idents []string, json []bool, replace bool) string {
	var b strings.Builder
	b.WriteString("LOAD DATA LOCAL INFILE 'Reader::")
	b.WriteString(reader)
	b.WriteString("' ")
	if replace {
		b.WriteString("REPLACE ")
	}
	b.WriteString("INTO TABLE ")
	b.WriteString(qualified)
	b.WriteString(" CHARACTER SET binary FIELDS TERMINATED BY '\\t' ESCAPED BY '\\\\' LINES TERMINATED BY '\\n' (")
	var sets []string
	for i, ident := range idents {
		if i > 0 {
			b.WriteString(", ")
		}
		if i < len(json) && json[i] {
			v := "@filament_" + strconv.Itoa(i)
			b.WriteString(v)
			sets = append(sets, ident+" = CONVERT("+v+" USING utf8mb4)")
			continue
		}
		b.WriteString(ident)
	}
	b.WriteString(")")
	if len(sets) > 0 {
		b.WriteString(" SET ")
		b.WriteString(strings.Join(sets, ", "))
	}
	return b.String()
}
