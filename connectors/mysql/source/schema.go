package mysql

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// rowDecoder appends one scanned row's text-protocol values into a RowWriter.
// Built once per table, shared read-only by its shards.
type rowDecoder struct {
	schema     rowmodel.Schema
	selectList string
	types      []mysqlType
	pks        []pkColumn
}

// decoderFor builds table's row decoder from information_schema.
func (s *Source) decoderFor(ctx context.Context, table string) (*rowDecoder, error) {
	cols, pks, err := s.tableMeta(ctx, table)
	if err != nil {
		return nil, err
	}
	return newRowDecoder(table, cols, pks), nil
}

// newRowDecoder classifies table's columns once: the RecordSchema it reports and
// the parser for each column.
func newRowDecoder(table string, cols []column, pks []pkColumn) *rowDecoder {
	d := &rowDecoder{
		schema: rowmodel.Schema{Resource: table, PrimaryKey: pkNames(pks), Engine: engine},
		types:  make([]mysqlType, len(cols)),
		pks:    pks,
	}
	parts := make([]string, len(cols))
	for i, c := range cols {
		t := typeFor(c.dataType, c.fullType)
		parts[i], d.types[i] = "t."+quoteIdent(c.name), t
		d.schema.Fields = append(d.schema.Fields, rowmodel.Field{
			Name: c.name, Nullable: c.nullable, Logical: t.logical, Native: c.fullType, Precision: t.precision, Scale: t.scale,
		})
	}
	d.selectList = strings.Join(parts, ", ")
	return d
}

// indexOf returns the positions of the named columns; a name the live table
// lacks is an error, since a key missing from the cursor would page forever.
func (d *rowDecoder) indexOf(names []string) ([]int, error) {
	idx := make([]int, len(names))
	for n, name := range names {
		idx[n] = slices.IndexFunc(d.schema.Fields, func(f rowmodel.Field) bool { return f.Name == name })
		if idx[n] < 0 {
			return nil, fmt.Errorf("column %q not in table %q", name, d.schema.Resource)
		}
	}
	return idx, nil
}

// appendRows drains a result set into w. Every value arrives as text (the
// driver's text protocol, or its rendering of a prepared statement's typed
// values); binary columns arrive as their raw bytes. keyIdx/keyCols name and type
// the columns encoded into each row's RowMeta.Key (nil for a keyless read).
// limit > 0 stops after that many rows. Returns the rows appended and last key.
func (d *rowDecoder) appendRows(rows *sql.Rows, w arrowbatch.RowWriter, keyIdx []int, keyCols []pkColumn, limit int) (int, []string, error) {
	raw := make([]sql.RawBytes, len(d.types))
	dest := make([]any, len(raw))
	for i := range raw {
		dest[i] = &raw[i]
	}
	n := 0
	var last []string
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return n, last, fmt.Errorf("scan: %w", err)
		}
		for i, t := range d.types {
			if raw[i] == nil {
				w.Null()
				continue
			}
			if err := t.parse(w, raw[i]); err != nil {
				return n, last, fmt.Errorf("column %q: %w", d.schema.Fields[i].Name, err)
			}
		}
		var meta rowmodel.Meta
		if keyIdx != nil {
			if len(keyIdx) != len(keyCols) {
				return n, last, fmt.Errorf("checkpoint key has %d columns but %d types", len(keyIdx), len(keyCols))
			}
			meta.Key = make([]string, len(keyIdx))
			for k, i := range keyIdx {
				meta.Key[k] = keyCols[k].checkpointValue(raw[i])
			}
		}
		if err := w.EndRow(meta); err != nil {
			return n, last, err
		}
		last = meta.Key
		n++
		if limit > 0 && n >= limit {
			break
		}
	}
	return n, last, rows.Err()
}

// column is one live column of a table: its name and information_schema DATA_TYPE
// (the bare type keyword, e.g. "varchar", "bigint"), plus COLUMN_TYPE (the full
// declaration, e.g. "bigint unsigned", kept in SchemaField.Native).
type column struct {
	name     string
	dataType string
	fullType string
	nullable bool
	pkOrder  int // 1-based position in the primary key; 0 = not a key column
}

// tableMeta returns the table's columns in ordinal order and its primary-key columns
// in key order, from one information_schema query.
func (s *Source) tableMeta(ctx context.Context, table string) ([]column, []pkColumn, error) {
	const q = `
SELECT c.COLUMN_NAME, c.DATA_TYPE, c.COLUMN_TYPE, c.IS_NULLABLE,
	COALESCE((
		SELECT k.ORDINAL_POSITION FROM information_schema.KEY_COLUMN_USAGE k
		WHERE k.TABLE_SCHEMA = c.TABLE_SCHEMA AND k.TABLE_NAME = c.TABLE_NAME
		  AND k.COLUMN_NAME = c.COLUMN_NAME AND k.CONSTRAINT_NAME = 'PRIMARY'
	), 0) AS pk_order
FROM information_schema.COLUMNS c
WHERE c.TABLE_SCHEMA = ? AND c.TABLE_NAME = ?
ORDER BY c.ORDINAL_POSITION`
	rows, err := s.db.QueryContext(ctx, q, s.database, table)
	if err != nil {
		return nil, nil, fmt.Errorf("columns %q: %w", table, err)
	}
	defer func() { _ = rows.Close() }()
	var cols []column
	for rows.Next() {
		var c column
		var isNullable string
		if err := rows.Scan(&c.name, &c.dataType, &c.fullType, &isNullable, &c.pkOrder); err != nil {
			return nil, nil, fmt.Errorf("scan column: %w", err)
		}
		c.nullable = isNullable == "YES"
		cols = append(cols, c)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(cols) == 0 {
		return nil, nil, fmt.Errorf("resource %q has no columns (missing table?)", table)
	}
	var pks []pkColumn
	maxOrder := 0
	for _, c := range cols {
		if c.pkOrder > maxOrder {
			maxOrder = c.pkOrder
		}
	}
	for ord := 1; ord <= maxOrder; ord++ {
		for _, c := range cols {
			if c.pkOrder == ord {
				// Keep COLUMN_TYPE rather than bare DATA_TYPE: unsigned integer
				// cursors must bind as UNSIGNED, and binary details are part of the
				// durable checkpoint's comparison contract.
				pks = append(pks, pkColumn{name: c.name, typ: c.fullType})
			}
		}
	}
	return cols, pks, nil
}

// pkColumn is one primary-key column: its raw name and information_schema
// COLUMN_TYPE, including qualifiers such as UNSIGNED and binary width. The type
// controls durable checkpoint encoding and SQL resume binding.
type pkColumn struct{ name, typ string }

const binaryCheckpointPrefix = "b64:"

// checkpointValue renders raw key bytes into a JSON-safe durable cursor. Textual
// MySQL keys already arrive in their comparison form; binary keys need an explicit
// encoding because encoding/json replaces invalid UTF-8 bytes in Go strings.
func (p pkColumn) checkpointValue(raw []byte) string {
	if isBinaryType(p.typ) {
		return binaryCheckpointPrefix + base64.RawStdEncoding.EncodeToString(raw)
	}
	return string(raw)
}

// bindValue reverses checkpointValue for SQL comparisons. A legacy binary
// checkpoint without the prefix is bound as its original bytes when possible.
func (p pkColumn) bindValue(value string) any {
	if isIntType(p.typ) && isUnsignedType(p.typ) {
		if n, err := strconv.ParseUint(value, 10, 64); err == nil {
			// Bind an actual uint64. MySQL's binary prepared-statement protocol
			// otherwise gives a string parameter signed comparison semantics at
			// values above MaxInt64 even inside CAST(... AS UNSIGNED).
			return n
		}
	}
	if !isBinaryType(p.typ) {
		return value
	}
	if encoded, ok := strings.CutPrefix(value, binaryCheckpointPrefix); ok {
		if decoded, err := base64.RawStdEncoding.DecodeString(encoded); err == nil {
			return decoded
		}
	}
	return []byte(value)
}

// quoteIdent renders s as a backtick-quoted MySQL identifier.
func quoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

// splitHostPort splits a go-sql-driver Addr ("host:port", port optional) for the
// replication client.
func splitHostPort(addr string) (string, uint16, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, fmt.Errorf("split %q: %w", addr, err)
	}
	n, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil || n == 0 {
		return "", 0, fmt.Errorf("invalid port in %q", addr)
	}
	return host, uint16(n), nil
}

// Schema returns the column schema of one table (COLUMN_TYPE kept verbatim in
// Native for a same-engine round trip, classified into a LogicalType) plus the
// primary key. It implements filament.SchemaProvider so a Schematized sink can
// build matching typed tables.
func (s *Source) Schema(ctx context.Context, resource string) (rowmodel.Schema, error) {
	cols, pks, err := s.tableMeta(ctx, resource)
	if err != nil {
		return rowmodel.Schema{}, err
	}
	return newRowDecoder(resource, cols, pks).schema, nil
}

// engine names this source in the schemas it reports.
const engine = "mysql"
