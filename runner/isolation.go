package runner

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/pipeline"
)

// failurePolicy builds the pipeline's failure policy from the run. Isolation
// is opt-in and never applies to CDC, whose shared stream position cannot
// skip one resource without losing its changes.
func failurePolicy(spec filament.RunSpec, plan filament.IngestionPlan, src filament.Source, sink filament.Sink) pipeline.FailurePolicy {
	if !spec.Options.IsolateResourceFailures || plan.RequiresCDC {
		return pipeline.FailurePolicy{}
	}
	isolatingSink, _ := sink.(filament.ResourceIsolatingSink)
	aware, _ := src.(filament.ResourceIsolatingSource)
	return pipeline.FailurePolicy{
		Isolate:                true,
		MaxFailedResources:     spec.Options.MaxFailedResources,
		SinkIsolation:          isolatingSink != nil && isolatingSink.IsolatesResources(),
		SourceStopsQuarantined: aware != nil && aware.IsolatesResourceFailures(),
	}
}

// allQuarantined reports whether no resource is left to commit.
func allQuarantined(resources []string, quarantined map[string]pipeline.Quarantine) bool {
	if len(quarantined) == 0 {
		return false
	}
	for _, resource := range resources {
		if _, ok := quarantined[resource]; !ok {
			return false
		}
	}
	return true
}

// errEveryResourceFailed builds the run error when isolation left nothing to
// commit, naming the first failed resource's cause.
func errEveryResourceFailed(quarantined map[string]pipeline.Quarantine) error {
	names := pipeline.QuarantinedResources(quarantined)
	first := names[0]
	return fmt.Errorf("every resource failed; %q: %w", first, quarantined[first].Err)
}

// destinationOf is the name the sink knows a resource by.
func destinationOf(spec filament.RunSpec, resource string) string {
	if policy, ok := spec.WritePolicies[resource]; ok && policy.DestinationResource != "" {
		return policy.DestinationResource
	}
	return resource
}

// discardedOnAbort reports whether aborting a failed resource leaves none of
// its rows in the destination: commit-gated writes are never committed, and a
// replacement is truncated or never promoted, as a whole-run Abort does.
// Apply-durable append, upsert and merge rows stay.
func discardedOnAbort(spec filament.RunSpec, resource string) bool {
	policy, ok := spec.WritePolicies[resource]
	if !ok {
		policy, ok = spec.WritePolicies[""]
	}
	return !ok || policy.Checkpoint == filament.CheckpointAfterCommit || policy.Capability.Mode == filament.WriteReplace
}

// settleQuarantine resolves quarantined resources before the sink commits the
// rest: it aborts each one's sink writes, then publishes its resource.failed.
// An isolating sink is told about every failed resource, written or not, so
// a commit-time replacement is never published empty over existing data. A
// sink that does not isolate only ever has unwritten resources quarantined.
// An abort that fails leaves the destination uncertain, so it fails the run.
func settleQuarantine(
	ctx context.Context,
	deps Deps,
	spec filament.RunSpec,
	sink filament.Sink,
	em *emitter,
	resources []string,
	quarantined map[string]pipeline.Quarantine,
) error {
	names := pipeline.QuarantinedResources(quarantined)
	isolating, _ := sink.(filament.ResourceIsolatingSink)
	sinkIsolates := isolating != nil && isolating.IsolatesResources()
	// A sink knows resources by destination name. Aborting a destination a
	// healthy resource also writes would discard that resource's rows while
	// the run reports it complete, so a shared destination cannot isolate.
	healthyDestinations := map[string]string{}
	for _, resource := range healthyResources(resources, quarantined) {
		healthyDestinations[destinationOf(spec, resource)] = resource
	}
	for _, resource := range names {
		if healthy, shared := healthyDestinations[destinationOf(spec, resource)]; shared {
			return fmt.Errorf("resource %q failed but shares destination %q with %q, so it cannot be isolated: %w",
				resource, destinationOf(spec, resource), healthy, quarantined[resource].Err)
		}
	}
	for _, resource := range names {
		if !quarantined[resource].Applied && !sinkIsolates {
			continue
		}
		aborter, ok := sink.(filament.ResourceIsolatingSink)
		if !ok {
			// The pipeline only quarantines an applied resource when the sink
			// declared isolation; reaching here is a contract violation.
			return fmt.Errorf("resource %q failed after Apply but sink %q cannot abort it", resource, spec.Sink.Connector)
		}
		abortCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), abortWait)
		err := aborter.AbortResource(abortCtx, destinationOf(spec, resource))
		cancel()
		if err != nil {
			return fmt.Errorf("abort failed resource %q: %w", resource, err)
		}
	}
	defer em.finish()()
	for _, resource := range names {
		discarded := discardedOnAbort(spec, resource)
		if discarded {
			em.dropResource(resource)
		}
		if deps.Log != nil {
			deps.Log.Warn("resource failed; run continues",
				filament.Field{Key: "event.name", Value: "runner.resource.isolated"},
				filament.Field{Key: "resource", Value: resource},
				filament.Field{Key: "discarded", Value: discarded},
				filament.Field{Key: "error", Value: quarantined[resource].Err.Error()})
		}
		emit(em, events.ResourceFailed, resource, events.ResourceFailedEvent{
			Error:     quarantined[resource].Err.Error(),
			Discarded: discarded,
		})
	}
	return nil
}

// failedWithCauses is emitter.failed for a run that had quarantined resources
// before it failed: each of those keeps its own cause.
func failedWithCauses(em *emitter, err error, resources []string, quarantined map[string]pipeline.Quarantine, resumable bool) {
	if len(quarantined) == 0 {
		em.failed(err, resources, resumable)
		return
	}
	defer em.finish()()
	em.schemaBlocked(err)
	for _, res := range resources {
		message := err.Error()
		if q, ok := quarantined[res]; ok {
			message = q.Err.Error()
		}
		emit(em, events.ResourceFailed, res, events.ResourceFailedEvent{Error: message})
	}
	if resumable {
		em.partial(err)
		return
	}
	em.fail(err)
}

// healthyResources returns resources minus the quarantined ones, in order.
func healthyResources(resources []string, quarantined map[string]pipeline.Quarantine) []string {
	out := make([]string, 0, len(resources))
	for _, resource := range resources {
		if _, ok := quarantined[resource]; !ok {
			out = append(out, resource)
		}
	}
	return out
}
