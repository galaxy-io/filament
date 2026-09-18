// Package streamcontrol connects the supported continuous profile to durable intake.
package streamcontrol

import (
	"context"
	"errors"
	"time"

	"github.com/galaxy-io/filament"
)

const (
	// LeaseTTL is the ownership lease duration for continuous worker attempts.
	LeaseTTL = 2 * time.Minute
	// DrainTimeout bounds graceful shutdown after a pause or stop request.
	DrainTimeout = 30 * time.Second
)

// Store includes the atomic admission and worker-claim operations required by
// the public continuous path. A basic StreamRuntimeStore alone is insufficient.
type Store interface {
	filament.DataStore
	filament.StreamRuntimeStore
	filament.ReplicationStreamRunStore
	ClaimStreamAttempt(context.Context, filament.LeaseToken) error
	RetireUnclaimedAttempt(context.Context, filament.LeaseToken) error
	PendingStreamRuns(context.Context, string, int) ([]filament.RunState, error)
	StreamProgress(context.Context, filament.TenantID, filament.RunID) (int64, int64, time.Time, error)
}

// Spec snapshots the submitted request without resolving secrets into persistence.
func Spec(s filament.RunState) filament.RunSpec {
	r := s.Request
	spec := filament.RunSpec{
		Tenant: r.Tenant, Run: s.Run, PipelineID: r.PipelineID, PipelineVersionID: r.PipelineVersionID,
		SourceConnectionID: r.SourceConnectionID, SinkConnectionID: r.SinkConnectionID, CheckpointRoute: r.CheckpointRoute,
		ReplicationStream: r.ReplicationStream, Source: r.Source, Sink: r.Sink, Resources: r.Resources, Selectors: r.Selectors,
		IngestionTypes: r.IngestionTypes, Options: r.Options, WorkerConfiguration: r.WorkerConfiguration,
	}
	spec.WritePolicies = map[string]filament.WritePolicy{}
	for _, resource := range r.Resources {
		spec.WritePolicies[resource] = filament.WritePolicy{Capability: filament.WriteCapabilities(filament.IngestionFullAppend)[0]}
	}
	return spec
}

// StateRequest extracts the tenant-scoped stream identity from an admitted run.
func StateRequest(s filament.RunState) (filament.StreamStateRequest, error) {
	if s.Request.ReplicationStream == nil {
		return filament.StreamStateRequest{}, errors.New("continuous run has no stream identity")
	}
	return filament.StreamStateRequest{Tenant: s.Tenant, Stream: *s.Request.ReplicationStream}, nil
}
