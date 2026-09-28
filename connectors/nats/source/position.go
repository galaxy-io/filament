package source

import (
	"fmt"

	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

// PositionCodec names JetStream's source stream sequence (not consumer sequence).
const PositionCodec = "nats.stream.sequence"

// RegisterCodec registers the connector's source-sequence interpretation.
func RegisterCodec(r *streamkit.Registry) error {
	return r.Register(PositionCodec, 0, streamkit.Uint64Codec{})
}

// Lookup resolves JetStream sequence positions without opening a connection.
func (*Source) Lookup(name string, version int) (rowmodel.PositionCodec, error) {
	if name != PositionCodec || version != 0 {
		return nil, fmt.Errorf("%w: %s v%d", streamkit.ErrUnknownCodec, name, version)
	}
	return streamkit.Uint64Codec{}, nil
}
