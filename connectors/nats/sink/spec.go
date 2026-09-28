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
			{Name: "stream", Type: filament.FieldString, Required: true, Scope: filament.ScopePipeline, Help: "Destination stream name: letters, digits, - and _, no dots. {resource} or {{resource}} expands to the destination resource name, e.g. EVENTS or {resource}_events"},
			{Name: "create_stream", Type: filament.FieldBool, Default: true, Scope: filament.ScopePipeline, Help: "Create missing streams"},
			{Name: "subject", Type: filament.FieldString, Required: true, Scope: filament.ScopePipeline, Help: "Publish subject captured by the stream. {resource} or {{resource}} expands to the destination resource name, e.g. events.{resource}"},
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
