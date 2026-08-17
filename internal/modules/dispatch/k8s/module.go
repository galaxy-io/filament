// Package k8s is the Kubernetes dispatch backend: it turns run.requested
// facts into worker Jobs.
package k8s

import (
	"context"
	"errors"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/runner"
)

// Module subscribes to run.requested and creates one worker Job per run.
type Module struct {
	cfg    Config
	ds     filament.DataStore
	client *client
	log    filament.Logger
	mx     filament.Metrics
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
	_ module.Module       = (*Module)(nil)
	_ filament.Dispatcher = (*Module)(nil)
)

// Name identifies this module.
func (m *Module) Name() string { return "dispatch" }

// Subscriptions declares a durable consumer over run.requested across every tenant/run.
func (m *Module) Subscriptions() []host.Subscription {
	return []host.Subscription{
		{Pattern: events.SubjectPattern(events.RunRequested), Durable: m.Name(), Handler: events.Handler(events.RunRequested, m.onRunRequested)},
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
	m.mx = d.Metrics
	m.client = c
	return nil
}

func (m *Module) onRunRequested(ctx context.Context, ev events.Event[events.RunRequestedEvent]) error {
	state, err := m.ds.LoadRun(ctx, ev.Run)
	if err != nil {
		return fmt.Errorf("k8sdispatch: load run %q: %w", ev.Run, err)
	}
	if !runner.ShouldRun(state) {
		return nil
	}
	_, err = m.Dispatch(ctx, runner.SpecFromState(state))
	return err
}

// Dispatch creates the worker Job for spec and returns a datastore-backed handle.
func (m *Module) Dispatch(ctx context.Context, spec filament.RunSpec) (filament.RunHandle, error) {
	if m.client == nil {
		return nil, errors.New("k8sdispatch: module is not mounted")
	}
	job, err := m.jobForSpec(spec)
	if err != nil {
		return nil, err
	}
	if err := m.client.createJob(ctx, m.cfg.Namespace, job); err != nil {
		if m.mx != nil {
			m.mx.Counter("filament_dispatch_failures_total").Inc()
		}
		return nil, err
	}
	if m.mx != nil {
		m.mx.Counter("filament_runs_dispatched_total").Inc()
	}
	if m.log != nil {
		m.log.Info("k8sdispatch: dispatched run",
			filament.Field{Key: "run", Value: string(spec.Run)},
			filament.Field{Key: "job", Value: job.Name},
			filament.Field{Key: "namespace", Value: m.cfg.Namespace},
		)
	}
	return runHandle{run: spec.Run, ds: m.ds}, nil
}
