package source

import (
	"github.com/galaxy-io/filament/streamkit"
)

// PositionCodec names JetStream's source stream sequence (not consumer sequence).
const PositionCodec = "nats.stream.sequence"

// RegisterCodec registers the connector's source-sequence interpretation.
func RegisterCodec(r *streamkit.Registry) error {
	return r.Register(PositionCodec, 0, streamkit.Uint64Codec{})
}
