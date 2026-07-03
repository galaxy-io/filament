package postgres

import (
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// protoMessage is the subset of generated message types marshalProtoSlice needs.
type protoMessage interface {
	proto.Message
}

// marshalProtoSlice encodes each element with protojson and wraps the results as a
// JSON array, suitable for a JSONB column.
func marshalProtoSlice[T protoMessage](items []T) ([]byte, error) {
	raw := make([]json.RawMessage, len(items))
	for i, item := range items {
		b, err := protojson.Marshal(item)
		if err != nil {
			return nil, err
		}
		raw[i] = b
	}
	return json.Marshal(raw)
}

func cloneProto(p *ingestionv1.Pipeline) *ingestionv1.Pipeline {
	return proto.Clone(p).(*ingestionv1.Pipeline)
}
