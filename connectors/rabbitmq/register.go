// Package rabbitmq registers the RabbitMQ Streams source and offset codec.
package rabbitmq

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/rabbitmq/source"
	"github.com/galaxy-io/filament/registry"
)

func init() {
	registry.RegisterPositionCodec(source.PositionCodec, 0, source.OffsetCodec{})
	registry.RegisterSource("rabbitmq", filament.MaturityAlpha, func() filament.Source { return source.New() })
}
