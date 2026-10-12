package sink

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/kafka/internal/client"
)

// Spec describes configuration and supported execution capabilities.
func (*Sink) Spec() filament.SinkSpec {
	policies := filament.WriteCapabilities(filament.IngestionFullAppend, filament.IngestionIncrementalAppend, filament.IngestionCDCAppend)
	for i := range policies {
		policies[i].Atomicity = filament.AtomicityRecord
		policies[i].Durability = filament.DurabilityAfterApply
	}
	fields := append(client.Fields(), []filament.ConfigField{
		{Name: "topic", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "Existing destination topic; use this or topic_prefix"},
		{Name: "topic_prefix", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "Publish to prefix.<destination resource>"},
		{Name: "format", Type: filament.FieldString, Default: "json", Scope: filament.ScopePipeline, Help: "json rows or message envelope passthrough"},
		{Name: "key_field", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "String or binary key column for JSON format"},
		{Name: "partition_field", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "Integer column selecting an existing partition"},
		{Name: "delete_mode", Type: filament.FieldString, Default: "record", Scope: filament.ScopePipeline, Help: "record or tombstone (nonnull key required)"},
		{Name: "publish_timeout", Type: filament.FieldString, Default: "30s", Scope: filament.ScopePipeline, Help: "Publication timeout of at least 1s"},
		{Name: "max_in_flight", Type: filament.FieldInt, Default: 1024, Scope: filament.ScopePipeline},
		{Name: "max_in_flight_bytes", Type: filament.FieldInt, Default: defaultMaxInFlightBytes, Scope: filament.ScopePipeline},
		{Name: "max_record_bytes", Type: filament.FieldInt, Default: 1 << 20, Scope: filament.ScopePipeline, Help: "512 bytes to 100 MiB, within max_in_flight_bytes"},
	}...)
	return filament.SinkSpec{Name: "kafka", DisplayName: client.DisplayName, DarkLogoURL: client.DarkLogoURL, LightLogoURL: client.LightLogoURL, Description: "Publish acknowledged append-only records to Kafka topics in bounded or continuous runs.", Version: "1", Maturity: filament.MaturityAlpha, Config: filament.ConfigSchema{Fields: fields}, Capabilities: filament.SinkCapabilities{EncodedIntegrity: true, PreferredBatchRows: 1000, PreferredBatchBytes: 4 << 20, WritePolicies: policies, Stream: &filament.StreamingSinkCapabilities{WritePolicies: []filament.WritePolicyCapability{{Mode: filament.WriteAppend, AcceptsOps: []filament.Operation{filament.OpInsert, filament.OpUpdate, filament.OpDelete}, Atomicity: filament.AtomicityRecord, Durability: filament.DurabilityAfterApply}}}}}
}
