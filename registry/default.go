package registry

import "github.com/galaxy-io/filament"

// DefaultSources and DefaultSinks are the process-wide registries that
// connectors populate from init(). A connector module does:
//
//	func init() {
//	    registry.RegisterSource("postgres", func() ingestion.Source { return NewSource() })
//	    registry.RegisterSink("postgres",   func() ingestion.Sink   { return NewSink() })
//	}
//
// The composition root blank-imports the connectors it ships and reads these
// defaults instead of hand-building a registry. Tests that want isolation
// construct their own via NewSources/NewSinks.
var (
	DefaultSources = NewSources()
	DefaultSinks   = NewSinks()
)

// RegisterSource registers a source factory on DefaultSources. It panics on a
// nil factory or a duplicate name (see Sources.Register).
func RegisterSource(name string, f ingestion.SourceFactory) {
	DefaultSources.Register(name, f)
}

// RegisterSink registers a sink factory on DefaultSinks. It panics on a nil
// factory or a duplicate name (see Sinks.Register).
func RegisterSink(name string, f ingestion.SinkFactory) {
	DefaultSinks.Register(name, f)
}
