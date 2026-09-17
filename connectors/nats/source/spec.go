package source

import "github.com/galaxy-io/filament"

func (*Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{Name: "nats", DisplayName: "NATS JetStream", Description: "Consume an existing dedicated JetStream durable consumer.", Version: "1", Config: filament.ConfigSchema{Fields: []filament.ConfigField{
		{Name: "url", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "NATS server URL"},
		{Name: "source_identity", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "Stable identity for this NATS account"},
		{Name: "credentials_file", Type: filament.FieldString, Scope: filament.ScopeConnection, Help: "Credentials file available on every worker"},
		{Name: "token", Type: filament.FieldSecret, Scope: filament.ScopeConnection},
		{Name: "stream", Type: filament.FieldString, Required: true, Scope: filament.ScopePipeline, Help: "JetStream stream to consume"},
		{Name: "consumer", Type: filament.FieldString, Required: true, Scope: filament.ScopePipeline, Help: "Existing unfiltered durable pull consumer: explicit ack, MaxAckPending=1, deliver-all"},
	}}, Stream: &filament.StreamCapabilities{Input: filament.InputMessages, Ordering: []filament.Ordering{filament.OrderingNone}, Delivery: filament.DeliveryReplayableAtLeastOnce}}
}
