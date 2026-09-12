package bigquery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"hash/crc32"
	"net/url"

	"cloud.google.com/go/bigquery"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
	parquetencoder "github.com/galaxy-io/filament/connectors/internal/parquet"
)

var encodedCRCTable = crc32.MakeTable(crc32.Castagnoli)

// Apply encodes one Arrow batch as Snappy Parquet, loads it into a short-lived
// BigQuery table, then inserts or merges its typed projection.
func (s *Sink) Apply(ctx context.Context, batch *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if s.client == nil {
		return filament.WriteReceipt{}, fmt.Errorf("bigquery sink: write before open")
	}
	if batch == nil || batch.Rows() == nil {
		return filament.WriteReceipt{}, fmt.Errorf("bigquery sink: write requires a record batch")
	}
	table, ok := s.tables[batch.Resource]
	if !ok {
		return filament.WriteReceipt{}, fmt.Errorf("bigquery sink: no schema ensured for resource %q", batch.Resource)
	}
	mode := opts.Policy.Capability.Mode
	if expected := s.modeFor(batch.Resource); expected != "" && mode != expected {
		return filament.WriteReceipt{}, fmt.Errorf("bigquery sink: apply policy %q does not match resource policy %q", mode, expected)
	}
	policy := opts.Policy
	if mode == filament.WriteReplace {
		policy.Capability.AcceptsOps = []filament.Operation{filament.OpInsert}
	}
	if err := policy.ValidateBatch(batch.Resource, batch); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("bigquery sink: %w", err)
	}

	switch mode {
	case filament.WriteAppend:
		return s.writeParquet(ctx, table, table.qualified, batch, false)
	case filament.WriteReplace:
		if table.replacement == "" {
			return filament.WriteReceipt{}, fmt.Errorf("bigquery sink: replacement table was not prepared for resource %q", batch.Resource)
		}
		return s.writeParquet(ctx, table, qualified(s.project, s.dataset, table.replacement), batch, false)
	case filament.WriteUpsert, filament.WriteDelete, filament.WriteMerge:
		if len(table.keys) == 0 {
			return filament.WriteReceipt{}, fmt.Errorf("bigquery sink: write policy %q requires a primary key for resource %q", mode, batch.Resource)
		}
		return s.writeParquet(ctx, table, table.qualified, batch, true)
	default:
		return filament.WriteReceipt{}, fmt.Errorf("bigquery sink: write policy %q is not implemented", mode)
	}
}

func (s *Sink) writeParquet(
	ctx context.Context,
	table tableDefinition,
	destination string,
	batch *arrowbatch.Batch,
	keyed bool,
) (filament.WriteReceipt, error) {
	encodedCRC := uint32(0)
	if batch.NumRows() == 0 {
		return s.receipt(batch, 0, batch.IntegrityCRC(), encodedCRC), nil
	}

	rows := batch.Rows()
	if keyed {
		rows = foldRecord(table, batch)
		defer rows.Release()
	}
	payload, err := encodeParquet(rows)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("bigquery sink: encode %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	encodedCRC = crc32.Checksum(payload, encodedCRCTable)
	writeCRC := batch.IntegrityCRC()

	stage := batchStage(s.project, s.dataset, table.qualified, s.run, batch.Part, batch.Seq)
	s.trackStage(stage)
	if err := s.loadParquet(ctx, stage, payload, batch.NumRows(), jobID(s.run, table.qualified, "load", batch.Part, batch.Seq)); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("bigquery sink: load stage for %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	// The stage is deleted after the typed write. Expiration is a fallback for
	// process termination, and must not make an otherwise retry-safe write fail.
	_ = s.execute(ctx, "ALTER TABLE "+qualified(s.project, s.dataset, stage)+
		" SET OPTIONS(expiration_timestamp = TIMESTAMP_ADD(CURRENT_TIMESTAMP(), INTERVAL 7 DAY))")

	statement := insertTableSQL(table, destination, qualified(s.project, s.dataset, stage))
	phase := "insert"
	if keyed {
		statement = mergeTableSQL(table, qualified(s.project, s.dataset, stage))
		phase = "merge"
	}
	if _, err := s.runQuery(ctx, statement, jobID(s.run, table.qualified, phase, batch.Part, batch.Seq)); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("bigquery sink: %s %s seq %d: %w", phase, batch.Resource, batch.Seq, err)
	}
	s.deleteStage(context.WithoutCancel(ctx), stage)
	return s.receipt(batch, int64(len(payload)), writeCRC, encodedCRC), nil
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

func (s *Sink) loadParquet(ctx context.Context, stage string, payload []byte, expectedRows int, id string) error {
	source := bigquery.NewReaderSource(bytes.NewReader(payload))
	source.SourceFormat = bigquery.Parquet
	loader := s.client.DatasetInProject(s.project, s.dataset).Table(stage).LoaderFrom(source)
	loader.CreateDisposition = bigquery.CreateIfNeeded
	loader.WriteDisposition = bigquery.WriteTruncate
	loader.DecimalTargetTypes = []bigquery.DecimalTargetType{
		bigquery.NumericTargetType, bigquery.BigNumericTargetType, bigquery.StringTargetType,
	}
	loader.JobID = id
	loader.Location = s.location
	status, err := s.runJob(ctx, id, loader.Run)
	if err != nil {
		return err
	}
	var statistics *bigquery.LoadStatistics
	if status.Statistics != nil {
		statistics, _ = status.Statistics.Details.(*bigquery.LoadStatistics)
	}
	if statistics != nil && statistics.OutputRows != int64(expectedRows) {
		return fmt.Errorf("load job wrote %d of %d rows", statistics.OutputRows, expectedRows)
	}
	return nil
}

func (s *Sink) runQuery(ctx context.Context, statement, id string) (*bigquery.JobStatus, error) {
	query := s.client.Query(statement)
	query.JobID = id
	query.Location = s.location
	return s.runJob(ctx, id, query.Run)
}

func (s *Sink) runJob(
	ctx context.Context,
	id string,
	start func(context.Context) (*bigquery.Job, error),
) (*bigquery.JobStatus, error) {
	job, startErr := start(ctx)
	if startErr != nil {
		var lookupErr error
		job, lookupErr = s.client.JobFromIDLocation(ctx, id, s.location)
		if lookupErr != nil {
			return nil, startErr
		}
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return nil, err
	}
	if err := status.Err(); err != nil {
		return nil, err
	}
	return status, nil
}

func (s *Sink) promoteReplacement(ctx context.Context, table tableDefinition) error {
	statement := replaceTableSQL(table, qualified(s.project, s.dataset, table.replacement))
	_, err := s.runQuery(ctx, statement, jobID(s.run, table.qualified, "replace", 0, 0))
	return err
}

func batchStage(project, dataset, destination string, run filament.RunID, part int, seq uint64) string {
	seed := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d\x00%d", project, dataset, destination, run, part, seq)
	hash := sha256.Sum256([]byte(seed))
	return fmt.Sprintf("_filament_load_%x", hash[:12])
}

func jobID(run filament.RunID, destination, phase string, part int, seq uint64) string {
	seed := fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%d", run, destination, phase, part, seq)
	hash := sha256.Sum256([]byte(seed))
	return fmt.Sprintf("filament_%s_%x", phase, hash[:16])
}

func destinationURI(project, dataset, resource string) string {
	return "bigquery://" + url.PathEscape(project) + "/" + url.PathEscape(dataset) + "/" + url.PathEscape(resource)
}

func (s *Sink) receipt(
	batch *arrowbatch.Batch,
	byteCount int64,
	writeCRC, encodedCRC uint32,
) filament.WriteReceipt {
	return filament.WriteReceipt{
		URI:        destinationURI(s.project, s.dataset, batch.Resource),
		Bytes:      byteCount,
		Rows:       batch.NumRows(),
		WriteCRC:   writeCRC,
		EncodedCRC: &encodedCRC,
	}
}
