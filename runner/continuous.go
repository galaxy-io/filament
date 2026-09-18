package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/pipeline"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

// ContinuousConfig configures an admitted serial attempt. RunOne handles only
// bounded runs. Schemas and append policies are pre-resolved.
type ContinuousConfig struct {
	Enabled                bool
	Spec                   filament.RunSpec
	Store                  filament.StreamRuntimeStore
	Source                 filament.Source
	Sink                   filament.Sink
	Codecs                 filament.CodecResolver
	Schemas                map[string]rowmodel.Schema
	Boundary               filament.Boundary
	LeaseTTL, DrainTimeout time.Duration
	// MaxEpochs bounds integration runs; zero runs until stopped or canceled.
	MaxEpochs int
}

// RunContinuous executes native epochs without publishing bounded lifecycle facts.
// Providers must honor cancellation. Forced cancellation abandons tentative work.
func RunContinuous(ctx context.Context, cfg ContinuousConfig) (result error) {
	if err := validateContinuous(cfg); err != nil {
		return err
	}
	spec := cfg.Spec
	dst := cfg.Sink.(filament.StreamingSink)
	lease := filament.LeaseToken{Tenant: spec.Tenant, Attempt: *spec.StreamAttempt}
	cleanupInstalled := false
	defer func() {
		if !cleanupInstalled {
			c, done := context.WithTimeout(context.WithoutCancel(ctx), cfg.DrainTimeout)
			defer done()
			result = errors.Join(result, cfg.Store.EndAttempt(c, filament.EndAttemptRequest{Lease: lease, Termination: filament.AttemptClean, Reason: continuousReason(result)}))
		}
	}()
	state, err := loadContinuousAttemptState(ctx, cfg, lease)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	var stopping atomic.Bool
	hb, err := startContinuousHeartbeat(ctx, cfg, lease, &stopping, cancel)
	if err != nil {
		return err
	}
	var session filament.StreamSession
	var p *pipeline.Pipeline
	var activeEpoch *filament.EpochRef
	opened := false
	defer func() {
		c, done := context.WithTimeout(context.WithoutCancel(ctx), cfg.DrainTimeout)
		defer done()
		if p != nil {
			p.CloseIngest(result)
			result = errors.Join(result, p.Wait())
		}
		var closeErr error
		if activeEpoch != nil {
			closeErr = dst.AbortEpoch(c, *activeEpoch)
		}
		if session != nil {
			closeErr = errors.Join(closeErr, session.Close(c))
		}
		if opened {
			closeErr = errors.Join(closeErr, dst.CloseSession(c))
		}
		closeErr = errors.Join(closeErr, cfg.Source.Teardown(c))
		hbErr := hb.Stop(c)
		termination := filament.AttemptClean
		if closeErr != nil || hbErr != nil {
			termination = filament.AttemptUnproven
		}
		endErr := cfg.Store.EndAttempt(c, filament.EndAttemptRequest{Lease: lease, Termination: termination, Reason: continuousReason(result)})
		result = errors.Join(result, closeErr, hbErr, endErr)
	}()
	cleanupInstalled = true
	if err := cfg.Source.Configure(runCtx, filament.NewConfig(spec.Source.Config)); err != nil {
		return err
	}
	if err := cfg.Sink.Open(runCtx, spec); err != nil {
		return err
	}
	opened = true
	if err := ensureContinuousSchemas(runCtx, cfg); err != nil {
		return err
	}
	session, err = cfg.Source.(filament.StreamSource).OpenStream(runCtx, filament.StreamOpenOpts{CheckAuthority: func(c context.Context) error { return cfg.Store.RenewLease(c, lease, cfg.LeaseTTL) }, Resources: spec.Resources, CommittedPositions: state.CommittedPositions.Clone(), Membership: state.Membership, Attempt: lease.Attempt})
	if err != nil {
		return err
	}
	p, err = pipeline.NewStream(pipeline.Config{Tenant: spec.Tenant, Run: spec.Run, Sink: cfg.Sink, WritePolicies: spec.WritePolicies, Options: spec.Options}, filament.OrderingNone)
	if err != nil {
		return err
	}
	p.Start(runCtx)
	return runEpochs(runCtx, cfg, p, session, dst, state, lease, &stopping, &activeEpoch)
}

func loadContinuousAttemptState(ctx context.Context, cfg ContinuousConfig, lease filament.LeaseToken) (filament.StreamState, error) {
	spec := cfg.Spec
	request := filament.StreamStateRequest{Tenant: spec.Tenant, Stream: filament.StreamRef{ID: lease.Attempt.StreamID, Generation: lease.Attempt.Generation}}
	state, err := cfg.Store.LoadStreamState(ctx, request)
	if err != nil {
		return filament.StreamState{}, err
	}
	if state.Attempt == nil || state.Attempt.Lease != lease || state.Desired != filament.StreamEnabled || state.PipelineVersionID != spec.PipelineVersionID || state.Run != spec.Run {
		return filament.StreamState{}, filament.ErrFenced
	}
	if err := cfg.Store.RenewLease(ctx, lease, cfg.LeaseTTL); err != nil {
		return filament.StreamState{}, err
	}
	return state, nil
}

// startContinuousHeartbeat renews ownership and observes pause/stop independently
// of source reads and sink writes. Only the heartbeat task owns drainStart.
func startContinuousHeartbeat(ctx context.Context, cfg ContinuousConfig, lease filament.LeaseToken, stopping *atomic.Bool, cancel context.CancelCauseFunc) (*streamkit.Heartbeat, error) {
	request := filament.StreamStateRequest{Tenant: lease.Tenant, Stream: filament.StreamRef{ID: lease.Attempt.StreamID, Generation: lease.Attempt.Generation}}
	var drainStart time.Time
	return streamkit.StartHeartbeat(context.WithoutCancel(ctx), min(cfg.LeaseTTL/3, time.Second), func(parent context.Context) error {
		c, done := context.WithTimeout(parent, cfg.LeaseTTL/3)
		defer done()
		err := cfg.Store.RenewLease(c, lease, cfg.LeaseTTL)
		if err == nil {
			var current filament.StreamState
			current, err = cfg.Store.LoadStreamState(c, request)
			if err == nil && current.Desired != filament.StreamEnabled {
				stopping.Store(true)
				if drainStart.IsZero() {
					drainStart = time.Now()
				}
				if time.Since(drainStart) >= cfg.DrainTimeout {
					err = context.DeadlineExceeded
				}
			}
		}
		if err != nil {
			cancel(err)
		}
		return err
	})
}

func validateContinuous(cfg ContinuousConfig) error {
	if !cfg.Enabled {
		return filament.ErrContinuousDisabled
	}
	spec := cfg.Spec
	if spec.Options.Execution.Normalize() != filament.ExecutionContinuous {
		return errors.New("continuous runner requires continuous execution")
	}
	if err := spec.ValidateStreamAttempt(); err != nil {
		return err
	}
	_, ok := cfg.Source.(filament.StreamSource)
	if !ok {
		return errors.New("continuous runner requires StreamSource")
	}
	_, ok = cfg.Sink.(filament.StreamingSink)
	if !ok {
		return errors.New("continuous runner requires StreamingSink")
	}
	if len(spec.Resources) == 0 || cfg.MaxEpochs < 0 || cfg.Store == nil || cfg.Codecs == nil || cfg.LeaseTTL < 30*time.Millisecond || cfg.LeaseTTL > 24*time.Hour || cfg.DrainTimeout <= 0 || cfg.Boundary.MaxWait <= 0 || cfg.Boundary.MaxRecords <= 0 {
		return errors.New("continuous runner: store, codecs, lease, drain and boundary limits required")
	}
	for _, r := range spec.Resources {
		policy, ok := spec.WritePolicies[r]
		if !ok {
			policy = spec.WritePolicies[""]
		}
		if policy.Capability.Mode != filament.WriteAppend {
			return errors.New("continuous runner: only append is supported")
		}
		if _, ok := cfg.Schemas[r]; !ok {
			return fmt.Errorf("continuous runner: missing schema for %s", r)
		}
	}
	return nil
}

func runEpochs(runCtx context.Context, cfg ContinuousConfig, p *pipeline.Pipeline, session filament.StreamSession, dst filament.StreamingSink, state filament.StreamState, lease filament.LeaseToken, stopping *atomic.Bool, activeEpoch **filament.EpochRef) error {
	spec := cfg.Spec
	var err error
	records := p.Records().(filament.StreamRecordSink)
	epoch := int64(1)
	if state.LastEpoch != nil {
		epoch = state.LastEpoch.Epoch + 1
	}
	for count := 0; cfg.MaxEpochs == 0 || count < cfg.MaxEpochs; count++ {
		if stopping.Load() {
			return nil
		}
		if err := context.Cause(runCtx); err != nil {
			return err
		}
		ref := filament.EpochRef{Attempt: lease.Attempt, Epoch: epoch, MembershipRevision: state.MembershipRevision}
		if err := p.BindEpoch(ref); err != nil {
			return err
		}
		if err := dst.BeginEpoch(runCtx, ref); err != nil {
			return err
		}
		*activeEpoch = &ref
		var coverage filament.Coverage
		for len(coverage.Positions) == 0 {
			coverage, err = session.Read(runCtx, records, cfg.Boundary)
			if err != nil {
				return err
			}
			if len(coverage.Claims) > 0 {
				return filament.ErrIncompleteCoverage
			}
			if stopping.Load() && len(coverage.Positions) == 0 {
				return nil
			}
		}
		proof, err := p.SealEpoch()
		if err != nil {
			return err
		}
		rows, nbytes := proof.Totals()
		commit := filament.EpochCommit{Completion: proof, Certificate: filament.EpochCertificate{FormatVersion: filament.EpochCertificateFormatVersion, Tenant: spec.Tenant, Ref: ref, PipelineVersionID: spec.PipelineVersionID, Coverage: coverage, Records: rows, Bytes: nbytes}}
		// Check source coverage before making destination effects durable.
		for resource, total := range proof.Resources() {
			commit.Certificate.Receipts = append(commit.Certificate.Receipts, filament.EpochReceipt{Resource: resource, Rows: total.Rows, Bytes: total.Bytes})
		}
		if err := commit.ValidateCompletion(cfg.Codecs); err != nil {
			return err
		}
		receipts, err := dst.CommitEpoch(runCtx, ref)
		if err != nil {
			return err
		}
		*activeEpoch = nil
		commit.Certificate.Receipts = receipts
		if err := commit.ValidateCompletion(cfg.Codecs); err != nil {
			return err
		}
		if _, err = cfg.Store.CommitEpoch(runCtx, commit); err != nil {
			historical, lookupErr := cfg.Store.GetEpoch(runCtx, filament.EpochLookup{Tenant: spec.Tenant, Key: ref.Key()})
			if lookupErr != nil {
				return errors.Join(err, lookupErr)
			}
			expected, e1 := commit.Certificate.CanonicalBytes(cfg.Codecs)
			actual, e2 := historical.Certificate.CanonicalBytes(cfg.Codecs)
			if e1 != nil || e2 != nil || !bytes.Equal(expected, actual) {
				return errors.Join(filament.ErrEpochConflict, e1, e2)
			}
		}
		// Historical lookup alone does not authorize acknowledgement.
		if err := cfg.Store.RenewLease(runCtx, lease, cfg.LeaseTTL); err != nil {
			return err
		}
		if err := session.Acknowledge(runCtx, coverage); err != nil {
			return err
		}
		epoch++
	}
	return nil
}

func ensureContinuousSchemas(runCtx context.Context, cfg ContinuousConfig) error {
	spec := cfg.Spec
	if schemaSink, ok := cfg.Sink.(filament.Schematized); ok {
		for _, r := range spec.Resources {
			if err := schemaSink.EnsureSchema(runCtx, r, cfg.Schemas[r]); err != nil {
				return err
			}
		}
	}
	return nil
}

func continuousReason(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
