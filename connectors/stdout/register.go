package stdout

import (
	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

// init registers the stdout sink on the default registry. Enable with:
//
//	import _ "github.com/galaxy-io/filament/connectors/stdout"
func init() {
	registry.RegisterSink("stdout", func() ingestion.Sink { return New() })
}
