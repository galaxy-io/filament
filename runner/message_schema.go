package runner

import (
	"context"
	"errors"
	"slices"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/streamproof"
	"github.com/galaxy-io/filament/schema"
)

// Decoder state travels with the existing durable epoch, not in an independent
// migration workflow. Legacy progress cannot establish a previously used layout.
func restoreMessageSchemas(ctx context.Context, cfg *ContinuousConfig, state filament.StreamState) error {
	if _, ok := cfg.Source.(filament.MessageSchemaSource); ok {
		if sink, ok := cfg.Sink.(filament.SchemaPolicySink); ok {
			cfg.messagePolicy = sink.SchemaPolicy()
		}
	}
	if cfg.messagePolicy != "" {
		if err := cfg.messagePolicy.Validate(); err != nil {
			return err
		}
	}
	if state.LastEpoch == nil {
		return nil
	}
	previous, err := cfg.Store.GetEpoch(ctx, filament.EpochLookup{Tenant: cfg.Spec.Tenant, Key: *state.LastEpoch})
	if err != nil {
		return err
	}
	cert := previous.Certificate
	if err := cert.ValidateMessageSchemas(); err != nil {
		return err
	}
	block := func(reason string) error { return &schema.BlockError{Reasons: []string{reason}} }
	if cert.MessageSchemas == nil {
		if cfg.messagePolicy != "" {
			return block("existing progress has no accepted message layouts; start a new stream generation")
		}
		return nil
	}
	if cfg.messagePolicy == "" {
		return block("cannot resume accepted message layouts without schema negotiation")
	}
	if cert.PipelineVersionID != cfg.Spec.PipelineVersionID {
		return block("message replay requires the same pipeline contract")
	}
	for resource := range cert.MessageSchemas {
		if !slices.Contains(cfg.Spec.Resources, resource) {
			return block("accepted message resource is no longer selected")
		}
	}
	cfg.messageSchemas = schema.CloneMessageLayouts(cert.MessageSchemas)
	return nil
}

func prepareContinuousSchemas(ctx context.Context, cfg *ContinuousConfig, state filament.StreamState) error {
	if err := restoreMessageSchemas(ctx, cfg, state); err != nil {
		return err
	}
	return ensureContinuousSchemas(ctx, cfg)
}

func messageEpochCommit(cfg ContinuousConfig, ref filament.EpochRef, coverage filament.Coverage, proof streamproof.Completion) filament.EpochCommit {
	rows, nbytes := proof.Totals()
	cert := filament.EpochCertificate{FormatVersion: filament.EpochCertificateFormatVersion, Tenant: cfg.Spec.Tenant, Ref: ref, PipelineVersionID: cfg.Spec.PipelineVersionID, Coverage: coverage, Records: rows, Bytes: nbytes}
	if cfg.messagePolicy != "" {
		cert.FormatVersion = filament.EpochMessageSchemaFormatVersion
		cert.MessageSchemas = proof.MessageSchemas()
	}
	return filament.EpochCommit{Completion: proof, Certificate: cert}
}

func openContinuousStream(ctx context.Context, cfg ContinuousConfig, opts filament.StreamOpenOpts) (filament.StreamSession, error) {
	if opts.MessageSchemaPolicy != "" {
		return cfg.Source.(filament.MessageSchemaSource).OpenMessageStream(ctx, opts)
	}
	return cfg.Source.(filament.StreamSource).OpenStream(ctx, opts)
}

func observeSchemaEvents(ctx context.Context, em *emitter) context.Context {
	if em == nil {
		return ctx
	}
	return schema.WithObserver(ctx, func(resource string, detection schema.Detection) {
		emit(em, events.SchemaDetected, resource, events.SchemaDetectedEvent(detection))
	})
}

func (e *emitter) schemaBlocked(err error) {
	var block *schema.BlockError
	if errors.As(err, &block) {
		emit(e, events.SchemaBlocked, block.Resource, events.SchemaBlockedEvent{Reasons: slices.Clone(block.Reasons)})
	}
}
