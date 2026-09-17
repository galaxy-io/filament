// Package nats registers the opt-in JetStream source. Default composition does
// not import this package until continuous compiler admission is available.
package nats

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/nats/source"
	"github.com/galaxy-io/filament/registry"
)

func init() {
	registry.RegisterSource("nats", filament.MaturityBeta, func() filament.Source { return source.New() })
}
