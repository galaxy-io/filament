package source

import (
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

func appendRecord(writer arrowbatch.RowWriter, projector *streamkit.Projector, columns *streamkit.MessageColumns, domain filament.DomainKey, r *kgo.Record) error {
	appendPayload, err := columns.Prepare(r.Value)
	if err != nil {
		return fmt.Errorf("kafka: topic %s partition %d offset %d: %w", r.Topic, r.Partition, r.Offset, err)
	}
	writer.String(r.Topic)
	writer.Int32(r.Partition)
	writer.Int64(r.Offset)
	writer.Bool(r.Value == nil)
	appendPayload(writer)
	var headers []streamkit.Header
	if r.Headers != nil {
		headers = make([]streamkit.Header, 0, len(r.Headers))
	}
	for _, h := range r.Headers {
		headers = append(headers, streamkit.Header{Key: h.Key, Value: h.Value, Null: h.Value == nil})
	}
	next := position(r.Offset + 1)
	envelope := streamkit.Envelope{Identity: filament.EventIdentity{Domain: domain, Position: next}, Timestamp: &r.Timestamp, Key: r.Key, KeyNull: r.Key == nil, Payload: r.Value, PayloadNull: r.Value == nil, Headers: headers}
	return projector.EndEvent(envelope, rowmodel.Meta{})
}
