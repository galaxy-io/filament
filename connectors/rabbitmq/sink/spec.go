package sink

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/rabbitmq/internal/connection"
)

// Spec describes connection settings and append policies for bounded and
// continuous runs.
func (*Sink) Spec() filament.SinkSpec {
	policies := filament.WriteCapabilities(filament.IngestionFullAppend, filament.IngestionIncrementalAppend, filament.IngestionCDCAppend)
	for i := range policies {
		policies[i].Atomicity = filament.AtomicityRecord
		policies[i].Durability = filament.DurabilityAfterApply
	}
	policies = append([]filament.WritePolicyCapability{{Mode: filament.WriteAppend, AcceptsOps: []filament.Operation{filament.OpInsert, filament.OpUpdate, filament.OpDelete}, Atomicity: filament.AtomicityRecord, Durability: filament.DurabilityAfterApply}}, policies...)
	fields := append(connection.Fields(), []filament.ConfigField{
		{Name: "stream", Type: filament.FieldString, Required: true, Scope: filament.ScopePipeline, Help: "Existing RabbitMQ stream. {resource} or {{resource}} expands to the destination resource name"},
		{Name: "publish_timeout", Type: filament.FieldString, Default: "30s", Scope: filament.ScopePipeline, Help: "Maximum wait for each publish confirmation, at least 1s"},
		{Name: "max_in_flight", Type: filament.FieldInt, Default: defaultMaxInFlight, Scope: filament.ScopePipeline, Help: "Maximum outstanding messages, from 1 through 65536"},
		{Name: "max_in_flight_bytes", Type: filament.FieldInt, Default: defaultMaxInFlightBytes, Scope: filament.ScopePipeline, Help: "Maximum outstanding message bytes"},
		{Name: "max_record_bytes", Type: filament.FieldInt, Default: 1 << 20, Scope: filament.ScopePipeline, Help: "Maximum JSON body size, from 512 bytes to 100 MiB"},
	}...)
	return filament.SinkSpec{
		Name: "rabbitmq", DisplayName: connection.DisplayName, Version: "1", IsPreview: true,
		DarkLogoURL: connection.DarkLogoURL, LightLogoURL: connection.LightLogoURL,
		Description: "Publish acknowledged JSON messages to an existing RabbitMQ stream in bounded or continuous runs.",
		Config:      filament.ConfigSchema{Fields: fields},
		Capabilities: filament.SinkCapabilities{
			EncodedIntegrity: true, PreferredBatchRows: 1000, PreferredBatchBytes: 4 << 20,
			WritePolicies:   policies,
			ReusableSession: true,
		},
	}
}
