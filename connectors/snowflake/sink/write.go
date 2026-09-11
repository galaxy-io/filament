package snowflake

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"hash/crc32"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	gosnowflake "github.com/snowflakedb/gosnowflake/v2"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
	parquetencoder "github.com/galaxy-io/filament/connectors/internal/parquet"
	"github.com/galaxy-io/filament/rowmodel"
)

var encodedCRCTable = crc32.MakeTable(crc32.Castagnoli)

// write serializes one batch as a complete Parquet file, uploads it to the
// user's internal stage, and atomically copies that file into the table.
func (s *Sink) write(ctx context.Context, table tableDefinition, batch *arrowbatch.Batch) (filament.WriteReceipt, error) {
	encodedCRC := uint32(0)
	if batch.NumRows() == 0 {
		return s.receipt(batch, 0, encodedCRC), nil
	}

	parquet, err := encodeParquet(batch.Rows())
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: encode %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	encodedCRC = crc32.Checksum(parquet, encodedCRCTable)
	writeCRC := batch.IntegrityCRC()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: acquire connection for %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	defer func() { _ = conn.Close() }()

	directory, file := stagedFile(table, s.run, batch.Part, batch.Seq)
	defer removeStagedFile(conn, directory+file)
	if err := putParquet(ctx, conn, directory, file, parquet); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: stage %s seq %d: %w", batch.Resource, batch.Seq, err)
	}

	if err := copyParquet(ctx, conn, table, table.qualified, directory, file, batch.NumRows(), false); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: load %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	return s.receiptWithCRC(batch, int64(len(parquet)), writeCRC, encodedCRC), nil
}

// writeFold loads one batch into a session-local table, then reduces repeated
// keys to their final operation and applies the result with one MERGE statement.
func (s *Sink) writeFold(ctx context.Context, table tableDefinition, batch *arrowbatch.Batch) (filament.WriteReceipt, error) {
	encodedCRC := uint32(0)
	if batch.NumRows() == 0 {
		return s.receipt(batch, 0, encodedCRC), nil
	}

	rows := foldRecord(table, batch)
	defer rows.Release()
	parquet, err := encodeParquet(rows)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: encode fold %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	encodedCRC = crc32.Checksum(parquet, encodedCRCTable)
	writeCRC := batch.IntegrityCRC()

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: acquire fold connection for %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, table.createTempSQL); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: create temporary table for %s: %w", batch.Resource, err)
	}

	directory, file := stagedFile(table, s.run, batch.Part, batch.Seq)
	defer removeStagedFile(conn, directory+file)
	if err := putParquet(ctx, conn, directory, file, parquet); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: stage fold %s seq %d: %w", batch.Resource, batch.Seq, err)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: begin fold %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := copyParquet(ctx, tx, table, table.temporary, directory, file, batch.NumRows(), true); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: load fold %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	if _, err := tx.ExecContext(ctx, table.mergeSQL); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: merge %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	if err := tx.Commit(); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: commit fold %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	return s.receiptWithCRC(batch, int64(len(parquet)), writeCRC, encodedCRC), nil
}

func foldRecord(table tableDefinition, batch *arrowbatch.Batch) arrow.RecordBatch {
	n := batch.NumRows()
	operationBuilder := array.NewInt16Builder(memory.DefaultAllocator)
	defer operationBuilder.Release()
	ordinalBuilder := array.NewInt64Builder(memory.DefaultAllocator)
	defer ordinalBuilder.Release()
	operationBuilder.Reserve(n)
	ordinalBuilder.Reserve(n)
	for i := range n {
		operationBuilder.Append(int16(batch.Op(i)))
		ordinalBuilder.Append(int64(i))
	}
	operations := operationBuilder.NewArray()
	defer operations.Release()
	ordinals := ordinalBuilder.NewArray()
	defer ordinals.Release()

	base := batch.Rows()
	fields := append(base.Schema().Fields(),
		arrow.Field{Name: table.operation.name, Type: arrow.PrimitiveTypes.Int16},
		arrow.Field{Name: table.ordinal.name, Type: arrow.PrimitiveTypes.Int64},
	)
	metadata := base.Schema().Metadata()
	schema := arrow.NewSchema(fields, &metadata)
	columns := append(append([]arrow.Array(nil), base.Columns()...), operations, ordinals)
	return array.NewRecordBatch(schema, columns, int64(n))
}

func encodeParquet(rows arrow.RecordBatch) ([]byte, error) {
	enc, err := parquetencoder.NewEncoder(rows.Schema(), encoder.CompressionSnappy)
	if err != nil {
		return nil, err
	}
	payload, _, err := enc.EncodeBatch(nil, rows)
	if err != nil {
		return nil, err
	}
	payload, _, err = enc.Finalize(payload)
	return payload, err
}

func stagedFile(table tableDefinition, run filament.RunID, part int, seq uint64) (directory, file string) {
	tableHash := sha256.Sum256([]byte(table.qualified))
	runHash := sha256.Sum256([]byte(run))
	directory = fmt.Sprintf("@~/filament/%x/%x/", tableHash[:8], runHash[:8])
	file = fmt.Sprintf("part-%06d-seq-%020d.parquet", part, seq)
	return directory, file
}

func putParquet(ctx context.Context, conn *sql.Conn, directory, file string, payload []byte) error {
	// The driver uses the URI's basename as the staged object name and reads the
	// bytes from WithFilePutStream; no local file is created.
	//nolint:gosec // directory and file contain only generated identifiers and digits.
	statement := "PUT 'file:///" + file + "' " + directory +
		" AUTO_COMPRESS = FALSE SOURCE_COMPRESSION = NONE OVERWRITE = TRUE"
	rows, err := conn.QueryContext(gosnowflake.WithFilePutStream(ctx, bytes.NewReader(payload)), statement)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	result, ok, err := readStatementResult(rows)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("PUT returned no result")
	}
	status := normalizeStatus(result["STATUS"])
	if status != "UPLOADED" && status != "SKIPPED" {
		return resultError("PUT", status, result["MESSAGE"])
	}
	return nil
}

type sqlQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func copyParquet(
	ctx context.Context,
	queryer sqlQueryer,
	table tableDefinition,
	destination, directory, file string,
	expectedRows int,
	includeInternal bool,
) error {
	targets := make([]string, len(table.columns))
	values := make([]string, len(table.columns))
	for i, column := range table.columns {
		targets[i] = column.identifier
		source := "$1:" + column.identifier
		if column.logical == rowmodel.LogicalJSON {
			values[i] = "PARSE_JSON(" + source + "::TEXT)"
		} else {
			values[i] = source + "::" + column.typ
		}
	}
	if includeInternal {
		targets = append(targets, table.operation.identifier, table.ordinal.identifier)
		values = append(values,
			"$1:"+table.operation.identifier+"::NUMBER(38,0)",
			"$1:"+table.ordinal.identifier+"::NUMBER(38,0)",
		)
	}

	//nolint:gosec // identifiers are quoted; types and stage paths are generated internally.
	statement := "COPY INTO " + destination + " (" + strings.Join(targets, ", ") + ")" +
		" FROM (SELECT " + strings.Join(values, ", ") + " FROM " + directory + ")" +
		" FILES = ('" + file + "')" +
		" FILE_FORMAT = (TYPE = PARQUET USE_LOGICAL_TYPE = TRUE BINARY_AS_TEXT = FALSE)" +
		" ON_ERROR = ABORT_STATEMENT PURGE = TRUE"
	rows, err := queryer.QueryContext(ctx, statement)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	result, ok, err := readStatementResult(rows)
	if err != nil {
		return err
	}
	// Snowflake can return no row when load history has already consumed this
	// deterministic file name. That is a successful idempotent retry.
	if !ok {
		return nil
	}
	status := normalizeStatus(result["STATUS"])
	if status == "LOAD_SKIPPED" || status == "SKIPPED" {
		return nil
	}
	if status != "LOADED" {
		return resultError("COPY INTO", status, result["FIRST_ERROR"])
	}
	loaded, err := strconv.Atoi(result["ROWS_LOADED"])
	if err != nil {
		return fmt.Errorf("COPY INTO returned invalid rows_loaded %q", result["ROWS_LOADED"])
	}
	if loaded != expectedRows {
		return fmt.Errorf("COPY INTO loaded %d of %d rows", loaded, expectedRows)
	}
	return nil
}

// readStatementResult reads the first row from Snowflake's command result and
// indexes values by the returned column names. Both PUT and COPY INTO expose
// their status this way.
func readStatementResult(rows *sql.Rows) (map[string]string, bool, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, false, err
	}
	if !rows.Next() {
		return nil, false, rows.Err()
	}
	values := make([]sql.NullString, len(columns))
	destinations := make([]any, len(columns))
	for i := range values {
		destinations[i] = &values[i]
	}
	if err := rows.Scan(destinations...); err != nil {
		return nil, false, err
	}
	result := make(map[string]string, len(columns))
	for i, column := range columns {
		if values[i].Valid {
			result[strings.ToUpper(column)] = values[i].String
		}
	}
	return result, true, nil
}

func normalizeStatus(status string) string {
	return strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(status)), " ", "_")
}

func resultError(command, status, message string) error {
	if status == "" {
		status = "UNKNOWN"
	}
	if message == "" {
		return fmt.Errorf("%s returned status %s", command, status)
	}
	return fmt.Errorf("%s returned status %s: %s", command, status, message)
}

func removeStagedFile(conn *sql.Conn, location string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	//nolint:gosec // location is the user stage plus generated path components.
	_, _ = conn.ExecContext(ctx, "REMOVE "+location)
}

func destinationURI(database, schema, resource string) string {
	return "snowflake://" + url.PathEscape(database) + "/" + url.PathEscape(schema) + "/" + url.PathEscape(resource)
}

func (s *Sink) receipt(batch *arrowbatch.Batch, byteCount int64, encodedCRC uint32) filament.WriteReceipt {
	return s.receiptWithCRC(batch, byteCount, batch.IntegrityCRC(), encodedCRC)
}

func (s *Sink) receiptWithCRC(batch *arrowbatch.Batch, byteCount int64, writeCRC, encodedCRC uint32) filament.WriteReceipt {
	return filament.WriteReceipt{
		URI:        destinationURI(s.database, s.schema, batch.Resource),
		Bytes:      byteCount,
		Rows:       batch.NumRows(),
		WriteCRC:   writeCRC,
		EncodedCRC: &encodedCRC,
	}
}
