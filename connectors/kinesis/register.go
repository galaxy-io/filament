// Package kinesis registers the Amazon Kinesis sink.
package kinesis

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/kinesis/sink"
	"github.com/galaxy-io/filament/registry"
)

func init() {
	registry.RegisterSink("kinesis", true, func() filament.Sink { return sink.New() })
}
