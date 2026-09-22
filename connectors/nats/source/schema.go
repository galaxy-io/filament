package source

import (
	"context"
	"sort"

	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

// Schema returns the subject and public row-aligned event envelope.
func Schema(resource string) (rowmodel.Schema, error) {
	return streamkit.WithEnvelopeFields(rowmodel.Schema{Resource: resource, Fields: []rowmodel.Field{{Name: "subject", Logical: rowmodel.LogicalString}}})
}

func messageHeaders(msg *nats.Msg) []streamkit.Header {
	if msg.Header == nil {
		return nil
	}
	keys := make([]string, 0, len(msg.Header))
	for key := range msg.Header {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	headers := make([]streamkit.Header, 0)
	for _, key := range keys {
		for _, value := range msg.Header[key] {
			headers = append(headers, streamkit.Header{Key: key, Value: []byte(value)})
		}
	}
	return headers
}

// Schema returns the fixed message envelope and subject column for a stream.
func (s *Source) Schema(_ context.Context, resource string) (rowmodel.Schema, error) {
	return Schema(resource)
}
