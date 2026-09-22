package runner

import (
	"context"
	"fmt"
	"maps"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

// prepareSinkResources resolves output names once, before opening the sink. The
// source-facing spec keeps its identities and gains write-boundary mappings;
// the sink-facing copy uses output identities throughout. No connector needs
// to interpret destination labels.
func prepareSinkResources(ctx context.Context, src filament.Source, snk filament.Sink, spec *filament.RunSpec, schemas map[string]rowmodel.Schema) (filament.RunSpec, map[string]rowmodel.Schema, error) {
	var err error
	schemas, err = loadSinkSchemas(ctx, src, snk, spec.Resources, schemas)
	if err != nil {
		return filament.RunSpec{}, nil, err
	}
	policies := maps.Clone(spec.WritePolicies)
	if policies == nil {
		policies = make(map[string]filament.WritePolicy)
	}
	out := *spec
	out.Resources = make([]string, 0, len(spec.Resources))
	out.WritePolicies = make(map[string]filament.WritePolicy)
	out.IngestionTypes = make(map[string]filament.IngestionType)
	out.CursorConfigs = make(map[string]filament.ResourceCursorConfig)
	if policy, ok := policies[""]; ok {
		policy.DestinationResource = ""
		out.WritePolicies[""] = policy
	}
	if ingestion, ok := spec.IngestionTypes[""]; ok {
		out.IngestionTypes[""] = ingestion
	}
	seen := make(map[string]string)
	for _, resource := range spec.Resources {
		policy, ok := policies[resource]
		if !ok {
			policy = policies[""]
		}
		destination := policy.DestinationResource
		if destination == "" {
			if namer, ok := src.(filament.DestinationResourceNamer); ok {
				destination = namer.DestinationResource(resource)
			}
		}
		if destination == "" {
			destination = schemas[resource].DestinationResource
		}
		if destination == "" {
			destination = resource
		}
		if previous, exists := seen[destination]; exists && previous != resource {
			return filament.RunSpec{}, nil, fmt.Errorf("resources %q and %q share destination %q; provide distinct labels", previous, resource, destination)
		}
		seen[destination] = resource
		policy.DestinationResource = destination
		policies[resource] = policy
		policy.Resource = destination
		policy.DestinationResource = ""
		out.WritePolicies[destination] = policy
		out.Resources = append(out.Resources, destination)
		if ingestion, ok := spec.IngestionTypes[resource]; ok {
			out.IngestionTypes[destination] = ingestion
		}
		if cursor, ok := spec.CursorConfigs[resource]; ok {
			out.CursorConfigs[destination] = cursor
		}
	}
	spec.WritePolicies = policies
	return out, schemas, nil
}

func destinationSchema(resource string, schema rowmodel.Schema, policies map[string]filament.WritePolicy) (string, rowmodel.Schema) {
	destination := policies[resource].DestinationResource
	if destination == "" {
		destination = resource
	}
	schema = schema.Clone()
	schema.Resource = destination
	schema.DestinationResource = ""
	return destination, schema
}

// Epoch receipts refer to destination resources at the sink boundary. Durable
// certificates refer to the source membership; retain all destination evidence.
func sourceEpochReceipts(spec filament.RunSpec, receipts []filament.EpochReceipt) []filament.EpochReceipt {
	sources := make(map[string]string, len(spec.Resources))
	for _, resource := range spec.Resources {
		destination := spec.WritePolicies[resource].DestinationResource
		if destination == "" {
			destination = resource
		}
		sources[destination] = resource
	}
	out := append([]filament.EpochReceipt(nil), receipts...)
	for i := range out {
		if source, ok := sources[out[i].Resource]; ok {
			out[i].Resource = source
		}
	}
	return out
}
