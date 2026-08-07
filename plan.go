package filament

import (
	"context"
	"fmt"
	"slices"
)

// ResolveIngestionPlan validates a run's ingestion type against both connector
// specs — the source's narrowed by its connection config — and binds
// per-resource write policies, discovering primary keys where the policy
// requires them.
func ResolveIngestionPlan(ctx context.Context, src Source, snk Sink, spec RunSpec) (IngestionPlan, error) {
	ingestionType := spec.IngestionType.OrDefault()
	sourcePolicy := SourcePolicyForIngestion(ingestionType)
	writePolicy := WritePolicyForIngestion(ingestionType)

	sourcePolicies := EffectiveSourcePolicies(src, NewConfig(spec.Source.Config))
	if len(sourcePolicies) > 0 {
		if err := ValidateSourcePolicies(src.Spec().Name, sourcePolicies, ingestionType); err != nil {
			return IngestionPlan{}, err
		}
	} else if err := ValidateSourceIngestion(src.Spec(), ingestionType); err != nil {
		return IngestionPlan{}, err
	}
	if err := ValidateSinkIngestion(snk.Spec(), ingestionType); err != nil {
		return IngestionPlan{}, err
	}

	policies := map[string]WritePolicy{}
	if len(spec.Resources) == 0 {
		policies[""] = writePolicy
	} else {
		for _, resource := range spec.Resources {
			policy := writePolicy
			policy.Resource = resource
			if policy.Capability.RequiresPK {
				keys, err := PrimaryKeyForResource(ctx, src, resource)
				if err != nil {
					return IngestionPlan{}, err
				}
				if len(keys) == 0 {
					return IngestionPlan{}, fmt.Errorf("%s requested for resource %q but no primary key was discovered", ingestionType, resource)
				}
				policy.Keys = keys
			}
			policies[resource] = policy
		}
	}

	return IngestionPlan{
		Type:          ingestionType,
		SourcePolicy:  sourcePolicy,
		WritePolicies: policies,
		RequiresCDC:   ingestionType == IngestionCDC,
		RequiresPK:    writePolicy.Capability.RequiresPK,
	}, nil
}

// ValidateSourceIngestion reports whether the source spec can serve the
// read-side policy the ingestion type implies.
func ValidateSourceIngestion(spec ConnectorSpec, t IngestionType) error {
	if len(spec.SourcePolicies) > 0 {
		return ValidateSourcePolicies(spec.Name, spec.SourcePolicies, t)
	}
	if slices.Contains(spec.Modes, SourcePolicyForIngestion(t.OrDefault()).Mode) {
		return nil
	}
	return fmt.Errorf("source %q does not support replication mode %v required by ingestion policy", spec.Name, SourcePolicyForIngestion(t.OrDefault()).Mode)
}

// ValidateSourcePolicies reports whether an effective policy set can serve
// the read-side policy the ingestion type implies.
func ValidateSourcePolicies(name string, policies []SourcePolicy, t IngestionType) error {
	t = t.OrDefault()
	policy := SourcePolicyForIngestion(t)
	for _, candidate := range policies {
		if candidate.Mode == policy.Mode && acceptsOperations(candidate.EmitsOps, policy.EmitsOps) && (!policy.Ordered || candidate.Ordered) {
			return nil
		}
	}
	return fmt.Errorf("source %q does not support ingestion type %q", name, t)
}

// EffectiveSourcePolicies resolves the policies a source offers under cfg;
// sources without the PolicyNarrower contract keep their static spec.
func EffectiveSourcePolicies(src Source, cfg Config) []SourcePolicy {
	if narrower, ok := src.(PolicyNarrower); ok {
		return narrower.PoliciesFor(cfg)
	}
	return src.Spec().SourcePolicies
}

// IsCDCReplication reports whether an effective policy set describes a CDC
// connection: every declared policy reads the change stream.
func IsCDCReplication(policies []SourcePolicy) bool {
	if len(policies) == 0 {
		return false
	}
	for _, policy := range policies {
		if policy.Mode != ModeCDC {
			return false
		}
	}
	return true
}

// ValidateSinkIngestion reports whether the sink spec can serve the write-side
// policy the ingestion type implies. Any sink may serve append and replace;
// upsert falls back to the Upsertable capability.
func ValidateSinkIngestion(spec SinkSpec, t IngestionType) error {
	policy := WritePolicyForIngestion(t)
	for _, candidate := range spec.Capabilities.WritePolicies {
		if candidate.Mode == policy.Capability.Mode && (!policy.Capability.RequiresPK || candidate.RequiresPK) &&
			(!policy.Capability.RequiresOrder || candidate.RequiresOrder) && acceptsOperations(candidate.AcceptsOps, policy.Capability.AcceptsOps) {
			return nil
		}
	}
	switch policy.Capability.Mode {
	case WriteAppend, WriteReplace:
		return nil
	case WriteUpsert:
		if spec.Capabilities.Upsertable {
			return nil
		}
	}
	return fmt.Errorf("sink %q does not support write policy %q", spec.Name, policy.Capability.Mode)
}

// PrimaryKeyForResource resolves a resource's primary key from the source's
// schema or discovery catalog; nil when neither reports one.
func PrimaryKeyForResource(ctx context.Context, src Source, resource string) ([]string, error) {
	if schemas, ok := src.(SchemaProvider); ok {
		schema, err := schemas.Schema(ctx, resource)
		if err != nil {
			return nil, fmt.Errorf("schema for %q: %w", resource, err)
		}
		return schema.PrimaryKey, nil
	}
	if discoverable, ok := src.(Discoverable); ok {
		result, err := discoverable.Discover(ctx, DiscoverOpts{})
		if err != nil {
			return nil, fmt.Errorf("discover resources: %w", err)
		}
		for _, candidate := range result.Resources {
			if candidate.Name == resource {
				return candidate.PrimaryKey, nil
			}
		}
	}
	return nil, nil
}

func acceptsOperations(have, want []Operation) bool {
	if len(want) == 0 {
		return true
	}
	if len(have) == 0 {
		return false
	}
	set := make(map[Operation]bool, len(have))
	for _, op := range have {
		set[op] = true
	}
	for _, op := range want {
		if !set[op] {
			return false
		}
	}
	return true
}
