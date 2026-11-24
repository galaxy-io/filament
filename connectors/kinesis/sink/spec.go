package sink

import "github.com/galaxy-io/filament"

// Spec describes configuration and supported execution capabilities.
func (*Sink) Spec() filament.SinkSpec {
	policies := filament.WriteCapabilities(filament.IngestionFullAppend, filament.IngestionIncrementalAppend, filament.IngestionCDCAppend)
	for i := range policies {
		policies[i].Atomicity = filament.AtomicityRecord
		policies[i].Durability = filament.DurabilityAfterApply
	}
	// Keyless, unordered append for continuous runs.
	policies = append([]filament.WritePolicyCapability{{Mode: filament.WriteAppend, AcceptsOps: []filament.Operation{filament.OpInsert, filament.OpUpdate, filament.OpDelete}, Atomicity: filament.AtomicityRecord, Durability: filament.DurabilityAfterApply}}, policies...)
	iamCredentials := &filament.FieldCondition{Field: "auth_method", Values: []string{authMethodIAMCredentials}}
	return filament.SinkSpec{
		Name:         "kinesis",
		DisplayName:  "Amazon Kinesis",
		Description:  "Publish acknowledged append-only records to Amazon Kinesis data streams in bounded or continuous runs.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-kinesis-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-kinesis-light.svg",
		Version:      "1",
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "region", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "AWS region; defaults to the SDK's resolved region."},
			{Name: "endpoint", Type: filament.FieldString, Scope: filament.ScopeConnection, Help: "Custom Kinesis endpoint; defaults to AWS."},
			{Name: "auth_method", Type: filament.FieldEnum, Default: authMethodInstanceProfile, Scope: filament.ScopeConnection, Help: "How Filament authenticates to Kinesis.", Enum: []filament.EnumOption{
				{Value: authMethodInstanceProfile, Label: "Instance Profile"},
				{Value: authMethodIAMCredentials, Label: "IAM Credentials"},
			}},
			{Name: "access_key_id", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, VisibleWhen: iamCredentials, Help: "AWS access key ID."},
			{Name: "secret_access_key", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, VisibleWhen: iamCredentials, Help: "AWS secret access key."},
			{Name: "session_token", Type: filament.FieldSecret, Scope: filament.ScopeConnection, VisibleWhen: iamCredentials, Help: "Optional AWS session token for temporary credentials."},
			{Name: "stream", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "Existing destination stream; use this or stream_prefix"},
			{Name: "stream_prefix", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "Publish to prefix.<destination resource>"},
			{Name: "key_field", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "String column used as the partition key; defaults to a hash of the record"},
			{Name: "publish_timeout", Type: filament.FieldString, Default: "30s", Scope: filament.ScopePipeline, Help: "Publication timeout of at least 1s, including retries of throttled records"},
			{Name: "max_record_bytes", Type: filament.FieldInt, Default: defaultMaxRecordBytes, Scope: filament.ScopePipeline, Help: "Record and partition key limit; 512 bytes to 10 MiB, within the stream's maximum record size"},
		}},
		Capabilities: filament.SinkCapabilities{EncodedIntegrity: true, PreferredBatchRows: maxRequestRecords, PreferredBatchBytes: 4 << 20, WritePolicies: policies, ReusableSession: true},
	}
}
