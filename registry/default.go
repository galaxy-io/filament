package registry

import "github.com/galaxy-io/filament"

// DefaultSources and DefaultSinks are the process-wide registries that
// connectors populate from init(). A connector module does:
//
//	func init() {
//	    registry.RegisterSource("postgres", filament.MaturityBeta, func() filament.Source { return NewSource() })
//	    registry.RegisterSink("postgres", filament.MaturityBeta, func() filament.Sink { return NewSink() })
//	}
//
// The composition root blank-imports the connectors it ships and reads these
// defaults instead of hand-building a registry. Tests that want isolation
// construct their own via NewSources/NewSinks.
var (
	DefaultSources = NewSources()
	DefaultSinks   = NewSinks()
)

// RegisterSource registers a source factory and its catalog maturity on
// DefaultSources. It panics on an invalid maturity, a nil factory, or a
// duplicate name (see Sources.RegisterWithMaturity).
func RegisterSource(name string, maturity filament.ConnectorMaturity, f filament.SourceFactory) {
	DefaultSources.RegisterWithMaturity(name, maturity, f)
}

// RegisterSink registers a sink factory and its catalog maturity on
// DefaultSinks. It panics on an invalid maturity, a nil factory, or a duplicate
// name (see Sinks.RegisterWithMaturity).
func RegisterSink(name string, maturity filament.ConnectorMaturity, f filament.SinkFactory) {
	DefaultSinks.RegisterWithMaturity(name, maturity, f)
}
