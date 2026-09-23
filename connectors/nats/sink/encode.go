package sink

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	jsonencoder "github.com/galaxy-io/filament/connectors/internal/json"
	"github.com/galaxy-io/filament/rowmodel"
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

// encoded is payload accounting for one batch: CRC32C over concatenated JSON
// message bodies without NDJSON delimiters, and their total byte length.
type encoded struct {
	crc   uint32
	bytes int64
}

// encode validates the complete batch before its first destination effect.
func (s *Sink) encode(b *arrowbatch.Batch) ([]*nats.Msg, encoded, error) {
	subject, err := s.cfg.route(b.Resource, s.filters)
	if err != nil {
		return nil, encoded{}, err
	}
	if b.Operations().Len() != 0 && b.Operations().Len() != b.NumRows() {
		return nil, encoded{}, fmt.Errorf("nats sink: operation count differs from row count")
	}
	rows := b.Rows()
	enc := jsonencoder.NewEncoder(rows.Schema())
	var ids *array.String
	fields := rows.Schema().FieldIndices(rowmodel.EventIDField)
	if len(fields) > 0 {
		if len(fields) != 1 {
			return nil, encoded{}, fmt.Errorf("nats sink: duplicate event ID column")
		}
		var ok bool
		ids, ok = rows.Column(fields[0]).(*array.String)
		if !ok {
			return nil, encoded{}, fmt.Errorf("nats sink: event ID must be a string column")
		}
	}
	messages := make([]*nats.Msg, 0, b.NumRows())
	var acc encoded
	for i := range b.NumRows() {
		op := b.Op(i)
		if op != filament.OpInsert && op != filament.OpUpdate && op != filament.OpDelete {
			return nil, encoded{}, fmt.Errorf("nats sink: unsupported operation %d", op)
		}
		payload := enc.AppendRow(nil, rows, i)
		if !json.Valid(payload) {
			return nil, encoded{}, fmt.Errorf("nats sink: invalid JSON in row %d", i)
		}
		msg := &nats.Msg{Subject: subject, Data: payload, Header: nats.Header{}}
		msg.Header.Set("Content-Type", "application/json")
		msg.Header.Set("Filament-Operation", filament.OperationName(op))
		msg.Header.Set(nats.ExpectedStreamHdr, s.cfg.stream)
		if ids != nil && !ids.IsNull(i) && ids.Value(i) != "" {
			// The namespace excludes run, attempt, epoch, and batch identities so replay
			// keeps the same ID. Subject/resource distinguish fan-out of one source event.
			identity, _ := json.Marshal([]string{s.namespace, s.cfg.stream, subject, b.Resource, ids.Value(i)})
			sum := sha256.Sum256(identity)
			msg.Header.Set(nats.MsgIdHdr, hex.EncodeToString(sum[:]))
		}
		// Size includes the subject, which is conservative for the server's payload
		// limit. Headers (including expected stream and dedup ID) are already final.
		if int64(msg.Size()) > s.maxPayload {
			return nil, encoded{}, fmt.Errorf("nats sink: row %d exceeds maximum message size %d", i, s.maxPayload)
		}
		if int64(msg.Size()) > s.cfg.maxInFlightBytes {
			return nil, encoded{}, fmt.Errorf("nats sink: row %d exceeds max_in_flight_bytes %d", i, s.cfg.maxInFlightBytes)
		}
		acc.crc = crc32.Update(acc.crc, crcTable, payload)
		acc.bytes += int64(len(payload))
		messages = append(messages, msg)
	}
	return messages, acc, nil
}
