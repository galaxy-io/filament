package source

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/nats/internal/connection"
)

// Spec describes the source's configuration and streaming capabilities.
func (*Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name:         "nats",
		DisplayName:  connection.DisplayName,
		Description:  "Consume subject patterns such as orders.> with automatically managed durable consumers.",
		DarkLogoURL:  connection.DarkLogoURL,
		LightLogoURL: connection.LightLogoURL,
		Version:      "1",
		Config: filament.ConfigSchema{Fields: append(connection.Fields(), []filament.ConfigField{
			{Name: "subjects", Type: filament.FieldList, Scope: filament.ScopePipeline, Help: "Optional default subject resources, e.g. orders.> and products.>. Consumers are managed automatically."},
			{Name: "streams", Type: filament.FieldList, Scope: filament.ScopePipeline, Help: "Stream/consumer pairs; each stream requires an existing dedicated durable pull consumer. Use instead of stream and consumer.", Fields: []filament.ConfigField{
				{Name: "stream", Type: filament.FieldString, Required: true},
				{Name: "consumer", Type: filament.FieldString, Required: true},
			}},
			{Name: "stream", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "JetStream stream to consume"},
			{Name: "consumer", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "Existing unfiltered durable pull consumer: explicit ack, MaxAckPending=1, deliver-all"},
		}...)}, Stream: &filament.StreamCapabilities{Input: filament.InputMessages, Ordering: []filament.Ordering{filament.OrderingNone}, Delivery: filament.DeliveryReplayableAtLeastOnce},
	}
}
