package webhook

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

// The composition root supplies the inbox through stream.Sources.
func init() {
	registry.RegisterSource("webhook", true, func() filament.Source { return New() })
}
