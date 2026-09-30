package bigquery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"hash/crc32"
	"net/url"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

const maxArrowBytes = 8 << 20

var encodedCRCTable = crc32.MakeTable(crc32.Castagnoli)

type tableState struct {
	definition                          tableDefinition
	model                               rowmodel.Schema
	stage                               string
	mu                                  sync.Mutex
	queue                               *uploadQueue
	nextOrdinal                         int64
	rows                                int64
	stream                              string
	segment                             hash.Hash
	stageCreated, publishing, published bool
}
type uploadBatch struct {
	schema, data []byte
	rows         int64
}

func (b uploadBatch) size() int64 { return int64(len(b.schema) + len(b.data)) }

// Apply queues owned Arrow IPC messages. Its receipt verifies encoding, not
// destination durability: all write capabilities require successful Commit.
func (s *Sink) Apply(ctx context.Context, batch *arrowbatch.Batch, opts filament.ApplyOptions) (receipt filament.WriteReceipt, err error) {
	if s.client == nil || s.sealed.Load() {
		return receipt, fmt.Errorf("bigquery sink: write outside an open run")
	}
	if batch == nil || batch.Rows() == nil {
		return receipt, fmt.Errorf("bigquery sink: write requires a record batch")
	}
	if err = s.failure(); err != nil {
		return receipt, err
	}
	state, ok := s.tables[batch.Resource]
	if !ok {
		return receipt, fmt.Errorf("bigquery sink: no schema ensured for %q", batch.Resource)
	}
	mode := opts.Policy.Capability.Mode
	if expected := s.modeFor(batch.Resource); mode != expected {
		return receipt, fmt.Errorf("bigquery sink: policy %q differs from planned policy %q", mode, expected)
	}
	switch mode {
	case filament.WriteAppend, filament.WriteReplace:
	case filament.WriteUpsert, filament.WriteMerge, filament.WriteDelete:
		if len(state.definition.keys) == 0 {
			return receipt, fmt.Errorf("bigquery sink: %s requires primary keys", mode)
		}
	default:
		return receipt, fmt.Errorf("bigquery sink: unsupported policy %q", mode)
	}
	policy := opts.Policy
	if mode == filament.WriteReplace {
		policy.Capability.AcceptsOps = []filament.Operation{filament.OpInsert}
	}
	if err = policy.ValidateBatch(batch.Resource, batch); err != nil {
		return receipt, err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.segment == nil {
		state.segment = sha256.New()
	}
	// A partially queued Apply must poison the run; publishing its prefix would
	// otherwise silently lose the rest of the batch.
	defer func() {
		if err != nil {
			s.fail(err)
		}
	}()
	rows, err := stageRecord(state, batch)
	if err != nil {
		return receipt, err
	}
	defer rows.Release()
	schemaPayload := ipc.GetSchemaPayload(rows.Schema(), memory.DefaultAllocator)
	defer schemaPayload.Release()
	var schema bytes.Buffer
	if _, err = schemaPayload.WritePayload(&schema); err != nil {
		return receipt, err
	}
	if state.queue == nil {
		state.queue = newUploadQueue()
		s.workers.Add(1)
		go s.uploadTable(state)
	}
	// Both caller cancellation and worker failure must unblock queue pressure.
	queueCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, cancel)
	defer func() { stop(); cancel() }()
	started := time.Now()
	var queueWait time.Duration
	crc := crc32.New(encodedCRCTable)
	var byteCount int64
	err = encodeChunks(rows, schema.Bytes(), maxArrowBytes, func(part uploadBatch) error {
		waiting := time.Now()
		if err := s.memory.Acquire(queueCtx, part.size()); err != nil {
			return err
		}
		queueWait += time.Since(waiting)
		if err := queueCtx.Err(); err != nil {
			s.memory.Release(part.size())
			return err
		}
		state.queue.push(part)
		// Length framing makes the digest unambiguous. Include the exact staged
		// schema, operations, ordinals and values, not a weak receipt checksum.
		fmt.Fprintf(state.segment, "%d:", len(part.schema))
		_, _ = state.segment.Write(part.schema)
		fmt.Fprintf(state.segment, "%d:", len(part.data))
		_, _ = state.segment.Write(part.data)
		_, _ = crc.Write(part.schema)
		_, _ = crc.Write(part.data)
		byteCount += part.size()
		return nil
	})
	if err != nil {
		return receipt, fmt.Errorf("bigquery sink: encode/queue %s: %w", batch.Resource, err)
	}
	s.logTiming("queue", batch.Resource, started, "records", batch.NumRows(), "bytes", byteCount, "backpressure_ms", queueWait.Milliseconds())
	state.nextOrdinal += int64(batch.NumRows())
	if err := json.NewEncoder(state.segment).Encode(batch.Cursor); err != nil {
		return receipt, fmt.Errorf("bigquery sink: fingerprint checkpoint: %w", err)
	}
	encodedCRC := crc.Sum32()
	return filament.WriteReceipt{URI: destinationURI(s.project, s.dataset, batch.Resource), Rows: batch.NumRows(), Bytes: byteCount, WriteCRC: batch.IntegrityCRC(), EncodedCRC: &encodedCRC}, nil
}

func stageRecord(state *tableState, batch *arrowbatch.Batch) (arrow.RecordBatch, error) {
	base := batch.Rows()
	if int(base.NumCols()) != len(state.model.Fields) {
		return nil, fmt.Errorf("bigquery sink: batch column count differs from ensured schema")
	}
	fields := base.Schema().Fields()
	columns := append([]arrow.Array(nil), base.Columns()...)
	for i, field := range state.model.Fields {
		if fields[i].Name != field.Name {
			return nil, fmt.Errorf("bigquery sink: batch field %q differs from ensured field %q", fields[i].Name, field.Name)
		}
		if field.Logical != rowmodel.LogicalTime && field.Logical != rowmodel.LogicalDecimal {
			continue
		}
		if columns[i].DataType().ID() == arrow.STRING {
			continue
		}
		builder := array.NewStringBuilder(memory.DefaultAllocator)
		for row := 0; row < batch.NumRows(); row++ {
			if columns[i].IsNull(row) {
				builder.AppendNull()
			} else {
				if decimal, ok := columns[i].(*array.Decimal128); ok {
					builder.Append(decimal.Value(row).ToString(int32(field.Scale)))
				} else {
					builder.Append(columns[i].ValueStr(row))
				}
			}
		}
		converted := builder.NewArray()
		builder.Release()
		defer converted.Release()
		columns[i] = converted
		fields[i].Type = arrow.BinaryTypes.String
	}
	opBuilder := array.NewInt16Builder(memory.DefaultAllocator)
	ordinalBuilder := array.NewInt64Builder(memory.DefaultAllocator)
	defer opBuilder.Release()
	defer ordinalBuilder.Release()
	opBuilder.Reserve(batch.NumRows())
	ordinalBuilder.Reserve(batch.NumRows())
	for i := 0; i < batch.NumRows(); i++ {
		opBuilder.Append(int16(batch.Op(i)))
		ordinalBuilder.Append(state.nextOrdinal + int64(i))
	}
	ops, ordinals := opBuilder.NewArray(), ordinalBuilder.NewArray()
	defer ops.Release()
	defer ordinals.Release()
	fields = append(fields, arrow.Field{Name: state.definition.operation.name, Type: arrow.PrimitiveTypes.Int16}, arrow.Field{Name: state.definition.ordinal.name, Type: arrow.PrimitiveTypes.Int64})
	columns = append(columns, ops, ordinals)
	return array.NewRecordBatch(arrow.NewSchema(fields, nil), columns, base.NumRows()), nil
}

// Split by encoded size, not row count: a few very wide rows can exceed the
// Storage Write API's 10 MB request limit. Reserve headroom for the envelope.
func encodeChunks(rows arrow.RecordBatch, schema []byte, limit int, emit func(uploadBatch) error) error {
	if rows.NumRows() == 0 {
		return nil
	}
	payload, err := ipc.GetRecordBatchPayload(rows, ipc.WithAllocator(memory.DefaultAllocator))
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	_, err = payload.WritePayload(&buf)
	payload.Release()
	if err != nil {
		return err
	}
	if buf.Len()+len(schema) <= limit {
		return emit(uploadBatch{schema: schema, data: buf.Bytes(), rows: rows.NumRows()})
	}
	if rows.NumRows() == 1 {
		return fmt.Errorf("one row exceeds the %d-byte Arrow request limit", limit)
	}
	middle := rows.NumRows() / 2
	left, right := rows.NewSlice(0, middle), rows.NewSlice(middle, rows.NumRows())
	defer left.Release()
	defer right.Release()
	// Drop the oversized encoding before recursing.
	buf = bytes.Buffer{}
	if err := encodeChunks(left, schema, limit, emit); err != nil {
		return err
	}
	return encodeChunks(right, schema, limit, emit)
}

func (s *Sink) runQuery(ctx context.Context, statement, id string) (*bigquery.JobStatus, error) {
	job, err := s.client.JobFromIDLocation(ctx, id, s.location)
	if isHTTPCode(err, 404) {
		query := s.client.Query(statement)
		query.JobID = id
		query.Location = s.location
		job, err = query.Run(ctx)
		if err != nil {
			// Submission may have succeeded even when the response was lost.
			recovered, lookupErr := s.client.JobFromIDLocation(ctx, id, s.location)
			if lookupErr == nil {
				job, err = recovered, nil
			}
		}
	}
	if err != nil {
		return nil, err
	}
	// Job lookup is recovery only when the staged data AND checkpoint evidence
	// match. Re-extraction can change its extent or batching; fail closed rather
	// than treating an older successful query as publication of those new rows.
	config, err := job.Config()
	if err != nil {
		return nil, err
	}
	query, ok := config.(*bigquery.QueryConfig)
	marker, _, _ := strings.Cut(statement, "\n")
	if !ok || !strings.HasPrefix(marker, "-- filament-segment:") || !strings.HasPrefix(query.Q, marker+"\n") {
		return nil, fmt.Errorf("bigquery sink: publication segment mismatch; refusing to acknowledge different rows or checkpoints")
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return nil, err
	}
	return status, status.Err()
}
func jobID(run filament.RunID, destination, phase string, part int, seq uint64, identity string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%d\x00%s", run, destination, phase, part, seq, identity)))
	return fmt.Sprintf("filament_%s_%x", phase, hash[:16])
}
func destinationURI(project, dataset, resource string) string {
	return "bigquery://" + url.PathEscape(project) + "/" + url.PathEscape(dataset) + "/" + url.PathEscape(resource)
}
