package engine

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
)

func resolveIngestionPlan(ctx context.Context, src ingestion.Source, snk ingestion.Sink, spec ingestion.RunSpec) (ingestion.IngestionPlan, error) {
	ingestionType := spec.IngestionType.OrDefault()
	sourcePolicy := ingestion.SourcePolicyForIngestion(ingestionType)
	writePolicy := ingestion.WritePolicyForIngestion(ingestionType)

	if err := validateSourcePolicy(src.Spec(), sourcePolicy); err != nil {
		return ingestion.IngestionPlan{}, err
	}
	if err := validateSinkPolicy(snk, writePolicy); err != nil {
		return ingestion.IngestionPlan{}, err
	}

	policies := map[string]ingestion.WritePolicy{}
	if len(spec.Resources) == 0 {
		policies[""] = writePolicy
	} else {
		for _, resource := range spec.Resources {
			policy := writePolicy
			policy.Resource = resource
			if policy.Capability.RequiresPK {
				keys, err := primaryKeyForResource(ctx, src, resource)
				if err != nil {
					return ingestion.IngestionPlan{}, err
				}
				if len(keys) == 0 {
					return ingestion.IngestionPlan{}, fmt.Errorf("%s requested for resource %q but no primary key was discovered", ingestionType, resource)
				}
				policy.Keys = keys
			}
			policies[resource] = policy
		}
	}

	return ingestion.IngestionPlan{
		Type:          ingestionType,
		SourcePolicy:  sourcePolicy,
		WritePolicies: policies,
		RequiresCDC:   ingestionType == ingestion.IngestionCDC,
		RequiresPK:    writePolicy.Capability.RequiresPK,
	}, nil
}

func validateSourcePolicy(spec ingestion.ConnectorSpec, policy ingestion.SourcePolicy) error {
	for _, candidate := range spec.SourcePolicies {
		if candidate.Mode == policy.Mode && acceptsOperations(candidate.EmitsOps, policy.EmitsOps) && (!policy.Ordered || candidate.Ordered) {
			return nil
		}
	}
	for _, mode := range spec.Modes {
		if mode == policy.Mode {
			return nil
		}
	}
	return fmt.Errorf("source %q does not support replication mode %v required by ingestion policy", spec.Name, policy.Mode)
}

func validateSinkPolicy(snk ingestion.Sink, policy ingestion.WritePolicy) error {
	spec := snk.Spec()
	for _, candidate := range spec.Capabilities.WritePolicies {
		if candidate.Mode == policy.Capability.Mode && (!policy.Capability.RequiresPK || candidate.RequiresPK) &&
			(!policy.Capability.RequiresOrder || candidate.RequiresOrder) && acceptsOperations(candidate.AcceptsOps, policy.Capability.AcceptsOps) {
			return nil
		}
	}
	switch policy.Capability.Mode {
	case ingestion.WriteAppend, ingestion.WriteReplace:
		return nil
	case ingestion.WriteUpsert:
		if spec.Capabilities.Upsertable {
			return nil
		}
	}
	return fmt.Errorf("sink %q does not support write policy %q", spec.Name, policy.Capability.Mode)
}

func primaryKeyForResource(ctx context.Context, src ingestion.Source, resource string) ([]string, error) {
	if schemas, ok := src.(ingestion.SchemaProvider); ok {
		schema, err := schemas.Schema(ctx, resource)
		if err != nil {
			return nil, fmt.Errorf("schema for %q: %w", resource, err)
		}
		return schema.PrimaryKey, nil
	}
	if discoverable, ok := src.(ingestion.Discoverable); ok {
		result, err := discoverable.Discover(ctx, ingestion.DiscoverOpts{})
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

func acceptsOperations(have, want []ingestion.Operation) bool {
	if len(want) == 0 {
		return true
	}
	if len(have) == 0 {
		return false
	}
	set := make(map[ingestion.Operation]bool, len(have))
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
