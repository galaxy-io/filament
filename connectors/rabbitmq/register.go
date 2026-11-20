// Package rabbitmq registers the RabbitMQ Streams sink.
package rabbitmq

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/rabbitmq/sink"
	"github.com/galaxy-io/filament/registry"
)

func init() {
	registry.RegisterSink("rabbitmq", true, func() filament.Sink { return sink.New() })
}
