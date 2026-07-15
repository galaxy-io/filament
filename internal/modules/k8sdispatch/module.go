// Package k8sdispatch turns run.requested facts into Kubernetes Jobs.
package k8sdispatch

import (
	"context"
	"errors"
	"fmt"

	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/module"
)

const defaultDurable = "k8sdispatch"

// Module subscribes to run.requested and creates one worker Job per run.
type Module struct {
	cfg    Config
	ds     ingestion.DataStore
	client *client
	log    ingestion.Logger
}

// New returns an unmounted k8s dispatcher.
func New(cfg Config) *Module {
	return &Module{cfg: cfg}
}

// NewFromEnv returns an unmounted k8s dispatcher configured from environment.
func NewFromEnv() *Module {
	return New(ConfigFromEnv())
}

var (
	_ module.Module        = (*Module)(nil)
	_ ingestion.Dispatcher = (*Module)(nil)
)

// Name identifies this module.
func (m *Module) Name() string { return "k8sdispatch" }

// Subscriptions declares a durable consumer over run.requested across every tenant/run.
func (m *Module) Subscriptions() []host.Subscription {
	return []host.Subscription{
		{Pattern: events.SubjectPattern(events.RunRequested), Durable: defaultDurable, Handler: events.Handler(events.RunRequested, m.onRunRequested)},
	}
}

// Mount validates the config and builds the Kubernetes client.
func (m *Module) Mount(_ context.Context, d module.Deps) error {
	if d.DataStore == nil {
		return errors.New("k8sdispatch: datastore is required")
	}
	if err := m.cfg.validate(); err != nil {
		return err
	}
	c, err := newClient(m.cfg)
	if err != nil {
		return err
	}
	m.ds = d.DataStore
	m.log = d.Log
	m.client = c
	return nil
}

func (m *Module) onRunRequested(ctx context.Context, ev events.Event[events.RunRequestedEvent]) error {
	state, err := m.ds.LoadRun(ctx, ev.Run)
	if err != nil {
		return fmt.Errorf("k8sdispatch: load run %q: %w", ev.Run, err)
	}
	if state.Status != ingestion.RunRequested && state.Status != ingestion.RunPartial {
		return nil
	}
	_, err = m.Dispatch(ctx, specFromState(state))
	return err
}

// Dispatch creates the worker Job for spec and returns a datastore-backed handle.
func (m *Module) Dispatch(ctx context.Context, spec ingestion.RunSpec) (ingestion.RunHandle, error) {
	if m.client == nil {
		return nil, errors.New("k8sdispatch: module is not mounted")
	}
	job := m.jobForSpec(spec)
	if err := m.client.createJob(ctx, m.cfg.Namespace, job); err != nil {
		return nil, err
	}
	if m.log != nil {
		m.log.Info("k8sdispatch: dispatched run",
			ingestion.Field{Key: "run", Value: string(spec.Run)},
			ingestion.Field{Key: "job", Value: job.Name},
			ingestion.Field{Key: "namespace", Value: m.cfg.Namespace},
		)
	}
	return runHandle{run: spec.Run, ds: m.ds}, nil
}

func specFromState(s ingestion.RunState) ingestion.RunSpec {
	r := s.Request
	return ingestion.RunSpec{
		Tenant:        r.Tenant,
		Run:           s.Run,
		Source:        r.Source,
		Sink:          r.Sink,
		Resources:     r.Resources,
		Selectors:     r.Selectors,
		IngestionType: r.IngestionType.OrDefault(),
		Mode:          ingestion.ModeFull,
		Options:       r.Options,
	}
}
