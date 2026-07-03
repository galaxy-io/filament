// Package runner executes a single already-persisted run in-process. It is the
// execution path used by the ingestion-worker binary, which is dispatched as a
// Kubernetes Job per run.
package runner

import (
	"context"
	"errors"

	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/secret"
)

// Deps are the collaborators RunOne needs to execute a run.
type Deps struct {
	Bus       eventbus.Bus
	DataStore ingestion.DataStore
	Secrets   secret.Provider
	Sources   ingestion.SourceRegistry
	Sinks     ingestion.SinkRegistry
}

// SpecFromState projects a persisted RunState into the RunSpec RunOne executes.
func SpecFromState(s ingestion.RunState) ingestion.RunSpec {
	r := s.Request
	return ingestion.RunSpec{
		Tenant:        r.Tenant,
		Run:           s.Run,
		Source:        r.Source,
		Sink:          r.Sink,
		DataStore:     r.DataStore,
		Resources:     r.Resources,
		Selectors:     r.Selectors,
		IngestionType: r.IngestionType.OrDefault(),
		Mode:          ingestion.ModeFull,
		Options:       r.Options,
	}
}

// RunOne executes a single run to completion, folding facts back onto the bus.
//
// TODO (@atterpac to implement): this is the worker's execution engine and is
// currently a stub so the ingestion-worker binary compiles and containerizes.
func RunOne(ctx context.Context, deps Deps, spec ingestion.RunSpec) error {
	return errors.New("runner: RunOne not implemented")
}
