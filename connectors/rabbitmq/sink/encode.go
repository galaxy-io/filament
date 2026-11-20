package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	jsonencoder "github.com/galaxy-io/filament/connectors/internal/json"
	"github.com/galaxy-io/filament/rowmodel"
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

// encoded is payload accounting for one batch. CRC32C covers concatenated JSON
// bodies without delimiters.
type encoded struct {
	crc    uint32
	bytes  int64
	stream string
}

type prepared struct {
	messages []*amqp.AMQP10
	sizes    []int64
	encoded  encoded
}

func (s *Sink) encode(ctx context.Context, b *arrowbatch.Batch) (prepared, error) {
	var out prepared
	d, err := s.destination(ctx, b.Resource)
	if err != nil {
		return out, err
	}
	if b.Operations().Len() != 0 && b.Operations().Len() != b.NumRows() {
		return out, fmt.Errorf("rabbitmq sink: operation count differs from row count")
	}
	rows := b.Rows()
	sequences, err := sequenceColumn(rows)
	if err != nil {
		return out, err
	}
	queried, err := d.producer.GetLastPublishingId()
	watermark, err := watermarkFromQuery(queried, err)
	if err != nil {
		return out, fmt.Errorf("rabbitmq sink: last publishing id: %w", err)
	}
	ids, err := assignPublishingIDs(sequences, watermark, b.NumRows())
	if err != nil {
		return out, err
	}
	eventIDs, err := stringColumn(rows, rowmodel.EventIDField)
	if err != nil {
		return out, err
	}
	enc := jsonencoder.NewEncoder(rows.Schema())
	out.messages = make([]*amqp.AMQP10, 0, b.NumRows())
	out.sizes = make([]int64, 0, b.NumRows())
	out.encoded.stream = d.name
	for i := range b.NumRows() {
		op := b.Op(i)
		if op != filament.OpInsert && op != filament.OpUpdate && op != filament.OpDelete {
			return prepared{}, fmt.Errorf("rabbitmq sink: unsupported operation %d", op)
		}
		payload := enc.AppendRow(nil, rows, i)
		if !json.Valid(payload) {
			return prepared{}, fmt.Errorf("rabbitmq sink: invalid JSON in row %d", i)
		}
		if int64(len(payload)) > s.cfg.maxRecordBytes {
			return prepared{}, fmt.Errorf("rabbitmq sink: row %d exceeds max_record_bytes %d", i, s.cfg.maxRecordBytes)
		}
		msg := amqp.NewMessage(payload)
		msg.SetPublishingId(ids[i])
		msg.Properties = &amqp.MessageProperties{ContentType: "application/json"}
		props := map[string]any{"Filament-Operation": filament.OperationName(op)}
		eventID := ""
		if eventIDs != nil && !eventIDs.IsNull(i) {
			eventID = eventIDs.Value(i)
			if eventID != "" {
				props["Filament-Event-Id"] = eventID
			}
		}
		msg.ApplicationProperties = props
		// Publishing ids are the sequence or the resumed counter, never a hash of
		// the event id. The event id is metadata for consumers.
		size := int64(len(payload) + len(eventID) + len(filament.OperationName(op)) + 64)
		if size > s.cfg.maxInFlightBytes {
			return prepared{}, fmt.Errorf("rabbitmq sink: row %d exceeds max_in_flight_bytes %d", i, s.cfg.maxInFlightBytes)
		}
		out.encoded.crc = crc32.Update(out.encoded.crc, crcTable, payload)
		out.encoded.bytes += int64(len(payload))
		out.messages = append(out.messages, msg)
		out.sizes = append(out.sizes, size)
	}
	return out, nil
}

func sequenceColumn(rows arrow.RecordBatch) ([]int64, error) {
	indices := rows.Schema().FieldIndices(rowmodel.AuditSequenceField)
	if len(indices) == 0 {
		return nil, nil
	}
	if len(indices) != 1 {
		return nil, fmt.Errorf("rabbitmq sink: duplicate %s column", rowmodel.AuditSequenceField)
	}
	col, ok := rows.Column(indices[0]).(*array.Int64)
	if !ok {
		return nil, fmt.Errorf("rabbitmq sink: %s must be an int64 column", rowmodel.AuditSequenceField)
	}
	ids := make([]int64, int(rows.NumRows()))
	for i := range ids {
		if col.IsNull(i) {
			return nil, fmt.Errorf("rabbitmq sink: %s is null in row %d", rowmodel.AuditSequenceField, i)
		}
		ids[i] = col.Value(i)
	}
	return ids, nil
}

func stringColumn(rows arrow.RecordBatch, name string) (*array.String, error) {
	indices := rows.Schema().FieldIndices(name)
	if len(indices) == 0 {
		return nil, nil
	}
	if len(indices) != 1 {
		return nil, fmt.Errorf("rabbitmq sink: duplicate %s column", name)
	}
	col, ok := rows.Column(indices[0]).(*array.String)
	if !ok {
		return nil, fmt.Errorf("rabbitmq sink: %s must be a string column", name)
	}
	return col, nil
}
