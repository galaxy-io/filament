package sink

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"math"
	"strings"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	jsonencoder "github.com/galaxy-io/filament/connectors/internal/json"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

type encoded struct {
	crc   uint32
	bytes int64
}

func column(rows arrow.RecordBatch, name string, required bool) (arrow.Array, error) {
	indices := rows.Schema().FieldIndices(name)
	if len(indices) == 0 && !required {
		return nil, nil
	}
	if len(indices) != 1 {
		return nil, fmt.Errorf("kafka sink: expected one %s column", name)
	}
	return rows.Column(indices[0]), nil
}

func binaryValue(col arrow.Array, i int, stringsOK bool) ([]byte, error) {
	if col == nil {
		return nil, nil
	}
	switch a := col.(type) {
	case *array.Binary:
		if a.IsNull(i) {
			return nil, nil
		}
		return append([]byte{}, a.Value(i)...), nil
	case *array.String:
		if stringsOK {
			if a.IsNull(i) {
				return nil, nil
			}
			return []byte(a.Value(i)), nil
		}
	}
	return nil, fmt.Errorf("kafka sink: expected %s column", map[bool]string{true: "string or binary", false: "binary"}[stringsOK])
}

func stringValue(col arrow.Array, i int) (string, error) {
	if col == nil {
		return "", nil
	}
	a, ok := col.(*array.String)
	if !ok {
		return "", fmt.Errorf("kafka sink: expected string column")
	}
	if a.IsNull(i) {
		return "", nil
	}
	return a.Value(i), nil
}

func partitionValue(col arrow.Array, i int) (int32, error) {
	if col.IsNull(i) {
		return 0, fmt.Errorf("kafka sink: null partition")
	}
	var n int64
	switch a := col.(type) {
	case *array.Int8:
		n = int64(a.Value(i))
	case *array.Int16:
		n = int64(a.Value(i))
	case *array.Int32:
		n = int64(a.Value(i))
	case *array.Int64:
		n = a.Value(i)
	default:
		return 0, fmt.Errorf("kafka sink: partition must be a signed integer column")
	}
	if n < 0 || n > math.MaxInt32 {
		return 0, fmt.Errorf("kafka sink: partition out of range")
	}
	return int32(n), nil
}

func (s *Sink) encode(b *arrowbatch.Batch) ([]*kgo.Record, encoded, error) {
	topic, err := s.cfg.route(b.Resource)
	if err != nil {
		return nil, encoded{}, err
	}
	if b.Operations().Len() != 0 && b.Operations().Len() != b.NumRows() {
		return nil, encoded{}, fmt.Errorf("kafka sink: operation count differs from row count")
	}
	rows := b.Rows()
	enc := jsonencoder.NewEncoder(rows.Schema())
	ids, err := column(rows, rowmodel.EventIDField, false)
	if err != nil {
		return nil, encoded{}, err
	}
	var key, payload, headers, timestamp, partition arrow.Array
	if s.cfg.keyField != "" {
		key, err = column(rows, s.cfg.keyField, true)
		if err != nil {
			return nil, encoded{}, err
		}
	}
	if s.cfg.partitionField != "" {
		partition, err = column(rows, s.cfg.partitionField, true)
		if err != nil {
			return nil, encoded{}, err
		}
	}
	if s.cfg.format == "message" {
		targets := []struct {
			name string
			dest *arrow.Array
		}{{rowmodel.EventKeyField, &key}, {rowmodel.EventPayloadField, &payload}, {rowmodel.EventHeadersField, &headers}, {rowmodel.EventTimestampField, &timestamp}}
		for _, t := range targets {
			*t.dest, err = column(rows, t.name, true)
			if err != nil {
				return nil, encoded{}, err
			}
		}
	}
	records := make([]*kgo.Record, 0, b.NumRows())
	var acc encoded
	for i := range b.NumRows() {
		op := b.Op(i)
		if op != filament.OpInsert && op != filament.OpUpdate && op != filament.OpDelete {
			return nil, encoded{}, fmt.Errorf("kafka sink: unsupported operation %d", op)
		}
		r := &kgo.Record{Topic: topic}
		r.Key, err = binaryValue(key, i, s.cfg.format != "message")
		if err != nil {
			return nil, encoded{}, err
		}
		if partition != nil {
			r.Partition, err = partitionValue(partition, i)
			if err != nil {
				return nil, encoded{}, err
			}
		}
		if s.cfg.format == "message" {
			if err := encodeMessage(r, payload, headers, timestamp, i); err != nil {
				return nil, encoded{}, err
			}
		} else {
			r.Value = enc.AppendRow(nil, rows, i)
			if !json.Valid(r.Value) {
				return nil, encoded{}, fmt.Errorf("kafka sink: invalid JSON in row %d", i)
			}
			r.Headers = append(r.Headers, kgo.RecordHeader{Key: "Content-Type", Value: []byte("application/json")})
		}
		if op == filament.OpDelete && s.cfg.deleteMode == "tombstone" {
			if r.Key == nil {
				return nil, encoded{}, fmt.Errorf("kafka sink: tombstone requires a nonnull key")
			}
			r.Value = nil
		}
		r.Headers = append(r.Headers, kgo.RecordHeader{Key: "Filament-Operation", Value: []byte(filament.OperationName(op))})
		id, err := stringValue(ids, i)
		if err != nil {
			return nil, encoded{}, err
		}
		if id != "" {
			r.Headers = append(r.Headers, kgo.RecordHeader{Key: "Filament-Event-Id", Value: []byte(id)})
		}
		size := recordSize(r)
		if size > s.cfg.maxRecordBytes || size > s.cfg.maxInFlightBytes {
			return nil, encoded{}, fmt.Errorf("kafka sink: row %d exceeds record or outstanding byte limit", i)
		}
		acc.crc = crc32.Update(acc.crc, crcTable, r.Value)
		acc.bytes += int64(len(r.Value))
		records = append(records, r)
	}
	return records, acc, nil
}

func encodeMessage(r *kgo.Record, payload, headers, timestamp arrow.Array, i int) error {
	var err error
	r.Value, err = binaryValue(payload, i, false)
	if err != nil {
		return err
	}
	raw, err := binaryValue(headers, i, false)
	if err != nil {
		return err
	}
	if raw != nil {
		hs, err := streamkit.DecodeHeaders(raw)
		if err != nil {
			return err
		}
		for _, h := range hs {
			if strings.EqualFold(h.Key, "Filament-Operation") || strings.EqualFold(h.Key, "Filament-Event-Id") {
				return fmt.Errorf("kafka sink: reserved header collision %q", h.Key)
			}
			r.Headers = append(r.Headers, kgo.RecordHeader{Key: h.Key, Value: bytes.Clone(h.Value)})
		}
	}
	ts, err := stringValue(timestamp, i)
	if err != nil {
		return err
	}
	if ts != "" {
		r.Timestamp, err = time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return fmt.Errorf("kafka sink: invalid timestamp: %w", err)
		}
	}
	return nil
}
