package redshift

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
	parquetencoder "github.com/galaxy-io/filament/connectors/internal/parquet"
	"github.com/galaxy-io/filament/rowmodel"
)

var encodedCRCTable = crc32.MakeTable(crc32.Castagnoli)

const (
	targetParquetInputBytes = int64(64 << 20)
	parquetFileIncrement    = int64(8)
)

type encodedLoad struct {
	payloads  [][]byte
	checksums []uint32
	crc       uint32
	hash      string
	bytes     int64
}

// Apply stages one Parquet batch and its explicit manifest in S3, COPYs it to
// a session-local Redshift table, and applies the requested policy atomically.
func (s *Sink) Apply(ctx context.Context, batch *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if s.pool == nil || s.staging == nil {
		return filament.WriteReceipt{}, fmt.Errorf("redshift sink: write before open")
	}
	if batch == nil || batch.Rows() == nil {
		return filament.WriteReceipt{}, fmt.Errorf("redshift sink: write requires a record batch")
	}
	table, ok := s.tables[batch.Resource]
	if !ok {
		return filament.WriteReceipt{}, fmt.Errorf("redshift sink: no schema ensured for resource %q", batch.Resource)
	}
	mode := opts.Policy.Capability.Mode
	if expected := s.modeFor(batch.Resource); expected != "" && mode != expected {
		return filament.WriteReceipt{}, fmt.Errorf("redshift sink: apply policy %q does not match resource policy %q", mode, expected)
	}
	policy := opts.Policy
	if mode == filament.WriteReplace {
		policy.Capability.AcceptsOps = []filament.Operation{filament.OpInsert}
	}
	if err := policy.ValidateBatch(batch.Resource, batch); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("redshift sink: %w", err)
	}
	keyed := mode == filament.WriteUpsert || mode == filament.WriteDelete || mode == filament.WriteMerge
	if keyed && len(table.keys) == 0 {
		return filament.WriteReceipt{}, fmt.Errorf("redshift sink: write policy %q requires a primary key for resource %q", mode, batch.Resource)
	}
	switch mode {
	case filament.WriteAppend, filament.WriteReplace, filament.WriteUpsert, filament.WriteDelete, filament.WriteMerge:
	default:
		return filament.WriteReceipt{}, fmt.Errorf("redshift sink: write policy %q is not implemented", mode)
	}

	writeCRC := batch.IntegrityCRC()
	encodedCRC := uint32(0)
	if batch.NumRows() == 0 {
		return s.receipt(batch, 0, writeCRC, encodedCRC), nil
	}

	rows := batch.Rows()
	if keyed {
		rows = foldRecord(table, batch)
		defer rows.Release()
	}
	encoded, err := encodeParquetLoad(rows)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("redshift sink: encode %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	encodedCRC = encoded.crc
	payloadID := encoded.hash
	token := batchToken(table.qualified, s.run, batch.Part, batch.Seq)

	applied, err := s.applied(ctx, s.pool.QueryRow(ctx, "SELECT payload_hash FROM "+s.ledger+" WHERE token = $1 LIMIT 1", token), token, payloadID)
	if err != nil {
		return filament.WriteReceipt{}, err
	}
	if applied {
		return s.receipt(batch, encoded.bytes, writeCRC, encodedCRC), nil
	}

	objectPrefix := stageObjectPrefix(table.qualified, s.run, batch.Part, batch.Seq, payloadID)
	staged, err := s.staging.put(ctx, objectPrefix, encoded.payloads, encoded.checksums)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("redshift sink: stage %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	defer s.staging.cleanup(context.WithoutCancel(ctx), staged)

	if err := s.applyStaged(ctx, table, batch, mode, keyed, token, payloadID, staged); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("redshift sink: apply %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	return s.receipt(batch, encoded.bytes, writeCRC, encodedCRC), nil
}

func (s *Sink) applyStaged(
	ctx context.Context,
	table tableDefinition,
	batch *arrowbatch.Batch,
	mode filament.WriteMode,
	keyed bool,
	token, payloadID string,
	staged stagedBatch,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	stageName := quoteIdent("_filament_load_" + token[:20])
	foldName := quoteIdent("_filament_fold_" + token[:20])
	if _, err := tx.Exec(ctx, createTempTableSQL(stageName, table, keyed)); err != nil {
		return fmt.Errorf("create load table: %w", err)
	}
	for i, manifestURI := range staged.manifestURIs {
		if _, err := tx.Exec(ctx, s.copySQL(stageName, manifestURI)); err != nil {
			return fmt.Errorf("COPY manifest %d: %w", i, err)
		}
	}
	var loaded int
	if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM "+stageName).Scan(&loaded); err != nil {
		return fmt.Errorf("count loaded rows: %w", err)
	}
	if loaded != batch.NumRows() {
		return fmt.Errorf("COPY loaded %d of %d rows", loaded, batch.NumRows())
	}

	destination := table.qualified
	if mode == filament.WriteReplace {
		destination = table.replacement
	}
	// Serialize mutations of one destination. This also makes the replay-ledger
	// check authoritative even though Redshift primary keys are informational.
	if _, err := tx.Exec(ctx, "LOCK TABLE "+destination); err != nil {
		return fmt.Errorf("lock destination: %w", err)
	}
	applied, err := s.applied(ctx, tx.QueryRow(ctx, "SELECT payload_hash FROM "+s.ledger+" WHERE token = $1 LIMIT 1", token), token, payloadID)
	if err != nil {
		return err
	}
	if applied {
		if err := tx.Rollback(ctx); err != nil {
			return fmt.Errorf("discard replay stage: %w", err)
		}
		return nil
	}

	if keyed {
		if _, err := tx.Exec(ctx, createFoldTableSQL(foldName, stageName, table)); err != nil {
			return fmt.Errorf("fold keys: %w", err)
		}
		if _, err := tx.Exec(ctx, deleteKeysSQL(destination, foldName, table)); err != nil {
			return fmt.Errorf("delete matching keys: %w", err)
		}
		if mode != filament.WriteDelete {
			if _, err := tx.Exec(ctx, insertFromStageSQL(destination, foldName, table, true)); err != nil {
				return fmt.Errorf("insert folded rows: %w", err)
			}
		}
	} else if _, err := tx.Exec(ctx, insertFromStageSQL(destination, stageName, table, false)); err != nil {
		return fmt.Errorf("insert rows: %w", err)
	}

	if _, err := tx.Exec(ctx, "INSERT INTO "+s.ledger+
		" (token, payload_hash, run_id, destination, part, seq, rows) VALUES ($1, $2, $3, $4, $5, $6::DECIMAL(20,0), $7)",
		token, payloadID, string(s.run), table.qualified, batch.Part, strconv.FormatUint(batch.Seq, 10), batch.NumRows()); err != nil {
		return fmt.Errorf("record replay token: %w", err)
	}
	if keyed {
		if _, err := tx.Exec(ctx, "DROP TABLE "+foldName); err != nil {
			return fmt.Errorf("drop fold table: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, "DROP TABLE "+stageName); err != nil {
		return fmt.Errorf("drop load table: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *Sink) applied(_ context.Context, row pgx.Row, token, payloadID string) (bool, error) {
	var previous string
	if err := row.Scan(&previous); err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("redshift sink: inspect replay token: %w", err)
	}
	if previous != payloadID {
		return false, fmt.Errorf("redshift sink: replay token %s has payload %s, received %s", token, previous, payloadID)
	}
	return true, nil
}

func (s *Sink) copySQL(stage, manifestURI string) string {
	auth := "IAM_ROLE default"
	if s.iamRole != "default" {
		auth = "IAM_ROLE " + quoteLiteral(s.iamRole)
	}
	return "COPY " + stage + " FROM " + quoteLiteral(manifestURI) + " " + auth + " MANIFEST FORMAT AS PARQUET"
}

func createTempTableSQL(name string, table tableDefinition, keyed bool) string {
	definitions := make([]string, 0, len(table.columns)+2)
	for _, column := range table.columns {
		definitions = append(definitions, column.identifier+" "+column.stageType)
	}
	if keyed {
		definitions = append(definitions,
			table.operation.identifier+" SMALLINT NOT NULL",
			table.ordinal.identifier+" BIGINT NOT NULL",
		)
	}
	return "CREATE TEMP TABLE " + name + " (" + strings.Join(definitions, ", ") + ")"
}

func createFoldTableSQL(fold, stage string, table tableDefinition) string {
	projected := columnIdentifiers(table.columns)
	projected = append(projected, table.operation.identifier)
	return "CREATE TEMP TABLE " + fold + " AS SELECT " + strings.Join(projected, ", ") +
		" FROM " + stage + " AS s QUALIFY ROW_NUMBER() OVER (PARTITION BY " + strings.Join(table.keys, ", ") +
		" ORDER BY " + table.ordinal.identifier + " DESC) = 1"
}

func deleteKeysSQL(destination, fold string, table tableDefinition) string {
	joins := make([]string, len(table.keys))
	for i, key := range table.keys {
		joins[i] = quoteIdent(table.name) + "." + key + " = s." + key
	}
	return "DELETE FROM " + destination + " USING " + fold + " AS s WHERE " + strings.Join(joins, " AND ")
}

func insertFromStageSQL(destination, source string, table tableDefinition, filterDeletes bool) string {
	targets := columnIdentifiers(table.columns)
	values := make([]string, len(table.columns))
	for i, column := range table.columns {
		value := "s." + column.identifier
		if column.logical == rowmodel.LogicalJSON {
			value = "JSON_PARSE(" + value + ")"
		}
		values[i] = value
	}
	statement := "INSERT INTO " + destination + " (" + strings.Join(targets, ", ") + ") SELECT " +
		strings.Join(values, ", ") + " FROM " + source + " AS s"
	if filterDeletes {
		statement += fmt.Sprintf(" WHERE s.%s <> %d", table.operation.identifier, rowmodel.OpDelete)
	}
	return statement
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

func encodeParquetLoad(rows arrow.RecordBatch) (encodedLoad, error) {
	ranges := parquetFileRanges(rows.NumRows(), arrowbatch.Bytes(rows))
	encoded := encodedLoad{
		payloads:  make([][]byte, 0, len(ranges)),
		checksums: make([]uint32, 0, len(ranges)),
	}
	hasher := sha256.New()
	var size [8]byte
	for _, rowRange := range ranges {
		part := rows.NewSlice(rowRange[0], rowRange[1])
		payload, err := encodeParquet(part)
		part.Release()
		if err != nil {
			return encodedLoad{}, err
		}
		checksum := crc32.Checksum(payload, encodedCRCTable)
		encoded.payloads = append(encoded.payloads, payload)
		encoded.checksums = append(encoded.checksums, checksum)
		encoded.crc = crc32.Update(encoded.crc, encodedCRCTable, payload)
		encoded.bytes += int64(len(payload))
		binary.BigEndian.PutUint64(size[:], uint64(len(payload)))
		_, _ = hasher.Write(size[:])
		_, _ = hasher.Write(payload)
	}
	encoded.hash = hex.EncodeToString(hasher.Sum(nil))
	return encoded, nil
}

func parquetFileRanges(rows, bytes int64) [][2]int64 {
	if rows <= 0 {
		return nil
	}
	files := max(int64(1), (bytes+targetParquetInputBytes-1)/targetParquetInputBytes)
	files = ((files + parquetFileIncrement - 1) / parquetFileIncrement) * parquetFileIncrement
	files = min(files, rows)
	ranges := make([][2]int64, files)
	for i := range files {
		ranges[i] = [2]int64{rows * i / files, rows * (i + 1) / files}
	}
	return ranges
}

func batchToken(destination string, run filament.RunID, part int, seq uint64) string {
	seed := fmt.Sprintf("%s\x00%s\x00%d\x00%d", destination, run, part, seq)
	hash := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(hash[:])
}

func stageObjectPrefix(destination string, run filament.RunID, part int, seq uint64, payloadID string) string {
	destinationHash := sha256.Sum256([]byte(destination))
	runHash := sha256.Sum256([]byte(run))
	return fmt.Sprintf("redshift/%x/%x/part-%06d-seq-%020d-%s", destinationHash[:8], runHash[:8], part, seq, payloadID[:16])
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func (s *Sink) receipt(batch *arrowbatch.Batch, bytes int64, writeCRC, encodedCRC uint32) filament.WriteReceipt {
	return filament.WriteReceipt{
		URI:        fmt.Sprintf("redshift://%s/%s/%s", s.database, s.schema, batch.Resource),
		Bytes:      bytes,
		Rows:       batch.NumRows(),
		WriteCRC:   writeCRC,
		EncodedCRC: &encodedCRC,
	}
}
