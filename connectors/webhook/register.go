package webhook

import (
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/streamkit"
)

// The source itself is registered by the composition root, which supplies the
// inbox connection. See cmd/internal/connectors.
func init() {
	registry.RegisterPositionCodec(PositionCodec, 0, streamkit.Uint64Codec{})
}
