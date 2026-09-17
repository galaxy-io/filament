package runner

import (
	"context"
	"errors"
	"github.com/galaxy-io/filament"
	natssource "github.com/galaxy-io/filament/connectors/nats/source"
	"github.com/galaxy-io/filament/internal/streamcontrol"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
	"time"
)

// ExecuteContinuousAttempt executes a supervisor-admitted identity exactly once.
// It never allocates an attempt or falls back to a bounded lifecycle.
func ExecuteContinuousAttempt(ctx context.Context, deps Deps, spec filament.RunSpec) (err error) {
	store, ok := deps.DataStore.(streamcontrol.Store)
	if !ok {
		return filament.ErrContinuousDisabled
	}
	if err := spec.ValidateStreamAttempt(); err != nil {
		return err
	}
	lease := filament.LeaseToken{Tenant: spec.Tenant, Attempt: *spec.StreamAttempt}
	if err := store.ClaimStreamAttempt(ctx, lease); err != nil {
		return err
	}
	entered := false
	defer func() {
		if !entered {
			c, cancel := context.WithTimeout(context.WithoutCancel(ctx), streamcontrol.DrainTimeout)
			defer cancel()
			reason := ""
			if err != nil {
				reason = err.Error()
			}
			err = errors.Join(err, store.EndAttempt(c, filament.EndAttemptRequest{Lease: lease, Termination: filament.AttemptClean, Reason: reason}))
		}
	}()
	if err := ResolveConfigRefs(ctx, deps.Secrets, &spec); err != nil {
		return err
	}
	source, err := deps.Sources.Resolve(spec.Source.Connector)
	if err != nil {
		return err
	}
	sink, err := deps.Sinks.Resolve(spec.Sink.Connector)
	if err != nil {
		return err
	}
	if err := streamcontrol.ValidateProfile(deps.DataStore, source, sink); err != nil {
		return err
	}
	schemas := map[string]rowmodel.Schema{}
	for _, resource := range spec.Resources {
		schema, err := natssource.Schema(resource)
		if err != nil {
			return err
		}
		schemas[resource] = schema
	}
	codecs := &streamkit.Registry{}
	if err := natssource.RegisterCodec(codecs); err != nil {
		return err
	}
	cfg := ContinuousConfig{Enabled: true, Spec: spec, Store: store, Source: source, Sink: sink, Codecs: codecs, Schemas: schemas, Boundary: filament.Boundary{MaxRecords: 1, MaxWait: time.Second}, LeaseTTL: streamcontrol.LeaseTTL, DrainTimeout: streamcontrol.DrainTimeout}
	if err := validateContinuous(cfg); err != nil {
		return err
	}
	entered = true
	return RunContinuous(ctx, cfg)
}

// LoadContinuousAttempt binds a worker to the immutable execution dispatched to
// it, refusing stale Jobs even when they refer to the same logical run.
func LoadContinuousAttempt(ctx context.Context, store streamcontrol.Store, state filament.RunState, executionID string) (filament.RunSpec, error) {
	req, err := streamcontrol.StateRequest(state)
	if err != nil {
		return filament.RunSpec{}, err
	}
	current, err := store.LoadStreamState(ctx, req)
	if err != nil {
		return filament.RunSpec{}, err
	}
	if current.Run != state.Run || current.Attempt == nil || current.Attempt.Spec.ExecutionID != executionID || current.Attempt.EndedAt != nil {
		return filament.RunSpec{}, filament.ErrFenced
	}
	return current.Attempt.Spec, nil
}
