package engine

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
)

func resolveIngestionPlan(ctx context.Context, src filament.Source, snk filament.Sink, spec filament.RunSpec) (filament.IngestionPlan, error) {
	ingestionType := spec.IngestionType.OrDefault()
	sourcePolicy := filament.SourcePolicyForIngestion(ingestionType)
	writePolicy := filament.WritePolicyForIngestion(ingestionType)

	if err := validateSourcePolicy(src.Spec(), ingestionType); err != nil {
		return filament.IngestionPlan{}, err
	}
	if err := validateSinkPolicy(snk, ingestionType); err != nil {
		return filament.IngestionPlan{}, err
	}

	policies := map[string]filament.WritePolicy{}
	if len(spec.Resources) == 0 {
		policies[""] = writePolicy
	} else {
		for _, resource := range spec.Resources {
			policy := writePolicy
			policy.Resource = resource
			if policy.Capability.RequiresPK {
				keys, err := primaryKeyForResource(ctx, src, resource)
				if err != nil {
					return filament.IngestionPlan{}, err
				}
				if len(keys) == 0 {
					return filament.IngestionPlan{}, fmt.Errorf("%s requested for resource %q but no primary key was discovered", ingestionType, resource)
				}
				policy.Keys = keys
			}
			policies[resource] = policy
		}
	}

	return filament.IngestionPlan{
		Type:          ingestionType,
		SourcePolicy:  sourcePolicy,
		WritePolicies: policies,
		RequiresCDC:   ingestionType == filament.IngestionCDC,
		RequiresPK:    writePolicy.Capability.RequiresPK,
	}, nil
}

func validateSourcePolicy(spec filament.ConnectorSpec, ingestionType filament.IngestionType) error {
	if filament.SourceSupportsIngestion(spec, ingestionType) {
		return nil
	}
	policy := filament.SourcePolicyForIngestion(ingestionType)
	return fmt.Errorf("source %q does not support replication mode %v required by ingestion policy", spec.Name, policy.Mode)
}

func validateSinkPolicy(snk filament.Sink, ingestionType filament.IngestionType) error {
	spec := snk.Spec()
	if filament.SinkSupportsIngestion(spec, ingestionType) {
		return nil
	}
	policy := filament.WritePolicyForIngestion(ingestionType)
	return fmt.Errorf("sink %q does not support write policy %q", spec.Name, policy.Capability.Mode)
}

func primaryKeyForResource(ctx context.Context, src filament.Source, resource string) ([]string, error) {
	if schemas, ok := src.(filament.SchemaProvider); ok {
		schema, err := schemas.Schema(ctx, resource)
		if err != nil {
			return nil, fmt.Errorf("schema for %q: %w", resource, err)
		}
		return schema.PrimaryKey, nil
	}
	if discoverable, ok := src.(filament.Discoverable); ok {
		result, err := discoverable.Discover(ctx, filament.DiscoverOpts{})
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
