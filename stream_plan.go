package filament

import (
	"context"
	"fmt"
	"reflect"
	"slices"
)

// ContinuousWritePlan is the pure compatibility decision shared by admission,
// API validation, and worker startup. Keys are bound from live schemas later.
type ContinuousWritePlan struct {
	Policy   WritePolicy
	Ordering Ordering
}

// PlanContinuousWrite selects a sink's explicit epoch policy. Runtime limits
// belong here rather than being inferred from connector names or bounded modes.
func PlanContinuousWrite(source ConnectorSpec, sink SinkSpec, mode WriteMode) (ContinuousWritePlan, error) {
	fail := func(message string) (ContinuousWritePlan, error) {
		return ContinuousWritePlan{}, fmt.Errorf("continuous planning: %s", message)
	}
	input := source.Stream
	if input == nil || input.Delivery != DeliveryReplayableAtLeastOnce {
		return fail("source must provide replayable input")
	}
	ops := slices.Clone(input.EmitsOps)
	switch input.Input {
	case InputRows, InputMessages:
		if len(ops) == 0 {
			ops = []Operation{OpInsert}
		}
	case InputChanges:
		if len(ops) == 0 {
			return fail("change sources must declare emitted operations")
		}
	default:
		return fail("unsupported input semantics")
	}
	for _, op := range ops {
		if op != OpInsert && op != OpUpdate && op != OpDelete {
			return fail("unknown source operation")
		}
	}
	// Replacement requires a finite snapshot and a completion/swap protocol.
	if mode == WriteReplace {
		return fail("replacement requires bounded snapshot completion")
	}
	switch mode {
	case WriteAppend, WriteUpsert, WriteMerge, WriteDelete:
	default:
		return fail("unknown streaming write mode")
	}
	caps := sink.Capabilities.Stream
	if caps == nil {
		return fail("sink does not advertise streaming support")
	}
	for _, candidate := range caps.WritePolicies {
		if candidate.Mode != mode {
			continue
		}
		compatible := true
		for _, op := range ops {
			compatible = compatible && candidate.Accepts(op)
		}
		if !compatible {
			continue
		}
		ordering := OrderingNone
		if candidate.RequiresOrder {
			// RequiresOrder has no domain scope. Only a global guarantee safely
			// satisfies it; partition and transaction scheduling need richer contracts.
			if !slices.Contains(input.Ordering, OrderingGlobalStrict) {
				continue
			}
			ordering = OrderingGlobalStrict
		}
		if candidate.Durability != DurabilityAfterApply && candidate.Durability != DurabilityAfterCommit {
			continue
		}
		candidate.AcceptsOps = slices.Clone(candidate.AcceptsOps)
		policy := WritePolicy{Capability: candidate, Checkpoint: CheckpointAfterCommit}
		if mode == WriteUpsert {
			policy.Version.Strategy = VersionInsertOrder
		}
		return ContinuousWritePlan{Policy: policy, Ordering: ordering}, nil
	}
	return fail(fmt.Sprintf("sink %q has no compatible streaming policy for %q (operations, ordering, or durability)", sink.Name, mode))
}

// PlanContinuousRun resolves every resource from submitted write intent. The
// sink declaration is authoritative; stale or weakened submitted capabilities
// are rejected. It is safe to repeat this before opening I/O.
func PlanContinuousRun(source Source, sink Sink, spec RunSpec) (map[string]WritePolicy, Ordering, error) {
	if err := ValidateContinuousConnectors(source, sink); err != nil {
		return nil, "", err
	}
	policies := make(map[string]WritePolicy, len(spec.Resources))
	ordering := OrderingNone
	for _, resource := range spec.Resources {
		if resource == "" {
			return nil, "", fmt.Errorf("continuous planning: empty resource")
		}
		if _, exists := policies[resource]; exists {
			return nil, "", fmt.Errorf("continuous planning: duplicate resource %q", resource)
		}
		submitted, ok := spec.WritePolicies[resource]
		if !ok {
			submitted, ok = spec.WritePolicies[""]
		}
		mode := submitted.Capability.Mode
		if !ok {
			intent, found := spec.IngestionTypes[resource]
			if !found {
				intent = spec.IngestionTypes[""]
			}
			// Compatibility for persisted requests predating explicit stream policies.
			switch intent {
			case IngestionFullAppend:
				mode = WriteAppend
			case IngestionFullUpsert:
				mode = WriteUpsert
			case IngestionCDCAppend:
				mode = WriteAppend
			case IngestionCDCMerge:
				mode = WriteMerge
			default:
				return nil, "", fmt.Errorf("continuous planning: explicit write intent required for %q", resource)
			}
		}
		plan, err := PlanContinuousWrite(source.Spec(), sink.Spec(), mode)
		if err != nil {
			return nil, "", fmt.Errorf("resource %q: %w", resource, err)
		}
		if ok && !sameStreamCapability(submitted.Capability, plan.Policy.Capability) {
			return nil, "", fmt.Errorf("continuous planning: submitted policy for %q differs from the sink declaration", resource)
		}
		policy := plan.Policy
		policy.Resource = resource
		policies[resource] = policy
		if plan.Ordering == OrderingGlobalStrict {
			ordering = plan.Ordering
		}
	}
	return policies, ordering, nil
}

func sameStreamCapability(a, b WritePolicyCapability) bool {
	a.AcceptsOps = slices.Clone(a.AcceptsOps)
	b.AcceptsOps = slices.Clone(b.AcceptsOps)
	slices.Sort(a.AcceptsOps)
	slices.Sort(b.AcceptsOps)
	a.AcceptsOps = slices.Compact(a.AcceptsOps)
	b.AcceptsOps = slices.Compact(b.AcceptsOps)
	return reflect.DeepEqual(a, b)
}

// resolveContinuousIngestionPlan mirrors bounded resolution: negotiate both
// connector contracts, then bind resource keys using the configured source.
func resolveContinuousIngestionPlan(ctx context.Context, source Source, sink Sink, spec RunSpec) (IngestionPlan, error) {
	policies, ordering, err := PlanContinuousRun(source, sink, spec)
	if err != nil {
		return IngestionPlan{}, err
	}
	for resource, policy := range policies {
		if policy.Capability.RequiresPK {
			keys, err := PrimaryKeyForResource(ctx, source, resource)
			if err != nil {
				return IngestionPlan{}, err
			}
			if len(keys) == 0 {
				return IngestionPlan{}, fmt.Errorf("continuous planning: resource %q requires a primary key", resource)
			}
			policy.Keys = slices.Clone(keys)
		}
		policies[resource] = policy
	}
	return IngestionPlan{WritePolicies: policies, Ordering: ordering}, nil
}
