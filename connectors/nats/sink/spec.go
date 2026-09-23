package sink

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/nats/internal/connection"
)

// Spec describes connection settings and bounded and streaming append policies.
func (*Sink) Spec() filament.SinkSpec {
	policies := filament.WriteCapabilities(filament.IngestionFullAppend, filament.IngestionIncrementalAppend, filament.IngestionCDCAppend)
	for i := range policies {
		policies[i].Atomicity = filament.AtomicityRecord
		policies[i].Durability = filament.DurabilityAfterApply
	}
	return filament.SinkSpec{
		Name: "nats", DisplayName: connection.DisplayName, Version: "1",
		DarkLogoURL: connection.DarkLogoURL, LightLogoURL: connection.LightLogoURL,
		Description: "Publish JSON records to an existing JetStream stream with acknowledged, append-only delivery.",
		Config: filament.ConfigSchema{Fields: append(connection.Fields(), []filament.ConfigField{
			{Name: "stream", Type: filament.FieldString, Required: true, Scope: filament.ScopePipeline, Help: "Existing destination JetStream stream"},
			{Name: routingField, Type: filament.FieldEnum, Default: routingPrefix, Scope: filament.ScopePipeline, Enum: []filament.EnumOption{
				{Value: routingPrefix, Label: "Prefix per resource"},
				{Value: routingFixed, Label: "Fixed subject"},
			}, Help: "Publish each destination resource under a subject prefix, or every record to one fixed subject"},
			{Name: "subject_prefix", Type: filament.FieldString, Required: true, Scope: filament.ScopePipeline, VisibleWhen: &filament.FieldCondition{Field: routingField, Values: []string{routingPrefix}}, Help: "Publish to <prefix>.<destination resource>; no trailing dot. Must be captured by the stream"},
			{Name: "subject", Type: filament.FieldString, Required: true, Scope: filament.ScopePipeline, VisibleWhen: &filament.FieldCondition{Field: routingField, Values: []string{routingFixed}}, Help: "One subject for every record. Must be captured by the stream"},
			{Name: "max_in_flight", Type: filament.FieldInt, Default: defaultMaxInFlight, Scope: filament.ScopePipeline, Help: "Maximum outstanding messages; 1 through 65536"},
			{Name: "max_in_flight_bytes", Type: filament.FieldInt, Default: defaultMaxInFlightBytes, Scope: filament.ScopePipeline, Help: "Maximum outstanding message bytes including payload, headers, and subject; individual messages must fit"},
			{Name: "publish_timeout", Type: filament.FieldString, Default: "5s", Scope: filament.ScopePipeline, Help: "Maximum wait for each publish acknowledgment"},
		}...)},
		Capabilities: filament.SinkCapabilities{
			EncodedIntegrity: true, PreferredBatchRows: 1000, PreferredBatchBytes: 4 << 20,
			WritePolicies: policies,
			Stream:        &filament.StreamingSinkCapabilities{WritePolicies: []filament.WritePolicyCapability{{Mode: filament.WriteAppend, AcceptsOps: []filament.Operation{filament.OpInsert, filament.OpUpdate, filament.OpDelete}, Atomicity: filament.AtomicityRecord, Durability: filament.DurabilityAfterApply}}},
		},
	}
}
