// Package nats registers the JetStream source, sink, and position codec.
package nats

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/nats/sink"
	"github.com/galaxy-io/filament/connectors/nats/source"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/streamkit"
)

func init() {
	registry.RegisterSink("nats", false, func() filament.Sink { return sink.New() })
	registry.RegisterPositionCodec(source.PositionCodec, 0, streamkit.Uint64Codec{})
	registry.RegisterSource("nats", false, func() filament.Source { return source.New() })
}
