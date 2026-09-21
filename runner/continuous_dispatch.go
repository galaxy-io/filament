package runner

import (
	"context"
	"errors"
	"time"

	"github.com/galaxy-io/filament"
)

// ExecuteContinuousAttempt executes a supervisor-admitted identity exactly once.
// It never allocates an attempt or falls back to a bounded lifecycle.
func ExecuteContinuousAttempt(ctx context.Context, deps Deps, spec filament.RunSpec) (err error) {
	store, ok := deps.DataStore.(filament.ContinuousRunStore)
	if !ok {
		return filament.ErrContinuousDisabled
	}
	if spec.Options.Execution.Normalize() != filament.ExecutionContinuous {
		return errors.New("continuous dispatch requires continuous execution")
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
			c, cancel := context.WithTimeout(context.WithoutCancel(ctx), DefaultDrainTimeout)
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
	codecs, ok := source.(filament.StreamSource)
	if !ok {
		return errors.New("continuous source must resolve position codecs")
	}
	cfg := ContinuousConfig{Enabled: true, Spec: spec, Store: store, Source: source, Sink: sink, Codecs: codecs, Boundary: filament.Boundary{MaxRecords: 1, MaxWait: time.Second}, LeaseTTL: DefaultLeaseTTL, DrainTimeout: DefaultDrainTimeout}
	if err := validateContinuous(cfg); err != nil {
		return err
	}
	entered = true
	return RunContinuous(ctx, cfg)
}

// LoadContinuousAttempt binds a worker to the immutable execution dispatched to
// it, refusing stale Jobs even when they refer to the same logical run.
func LoadContinuousAttempt(ctx context.Context, store filament.StreamRuntimeStore, state filament.RunState, executionID string) (filament.RunSpec, error) {
	req, err := state.StreamStateRequest()
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
