package registry

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/streamkit"
)

// DefaultSources and DefaultSinks are the process-wide registries that
// connectors populate from init(). A connector module does:
//
//	func init() {
//	    registry.RegisterSource("postgres", true, func() filament.Source { return NewSource() })
//	    registry.RegisterSink("postgres", true, func() filament.Sink { return NewSink() })
//	}
//
// The composition root blank-imports the connectors it ships and reads these
// defaults instead of hand-building a registry. Tests that want isolation
// construct their own via NewSources/NewSinks.
var (
	// DefaultCodecs contains pure position codecs registered by shipped connectors.
	DefaultCodecs  = &streamkit.Registry{}
	DefaultSources = NewSources()
	DefaultSinks   = NewSinks()
)

// RegisterSource registers a source factory and its catalog preview flag on
// DefaultSources. It panics on a nil factory, or a
// duplicate name (see Sources.RegisterWithPreview).
func RegisterSource(name string, isPreview bool, f filament.SourceFactory) {
	DefaultSources.RegisterWithPreview(name, isPreview, f)
}

// RegisterSink registers a sink factory and its catalog preview flag on
// DefaultSinks. It panics on a nil factory, or a duplicate
// name (see Sinks.RegisterWithPreview).
func RegisterSink(name string, isPreview bool, f filament.SinkFactory) {
	DefaultSinks.RegisterWithPreview(name, isPreview, f)
}

// RegisterPositionCodec registers a connector-owned codec for durable progress.
// Like provider registration, duplicate or invalid registrations panic at startup.
func RegisterPositionCodec(name string, version int, codec filament.PositionCodec) {
	if err := DefaultCodecs.Register(name, version, codec); err != nil {
		panic(err)
	}
}
