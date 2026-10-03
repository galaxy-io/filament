// Package kafka registers the Kafka source and offset codec.
package kafka

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/kafka/source"
	"github.com/galaxy-io/filament/registry"
)

func init() {
	registry.RegisterPositionCodec(source.PositionCodec, 0, source.OffsetCodec{})
	registry.RegisterSource("kafka", filament.MaturityAlpha, func() filament.Source { return source.New() })
}
