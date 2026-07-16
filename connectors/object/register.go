package s3

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

// init registers the s3 object-store sink on the default registry. Enable with:
//
//	import _ "github.com/galaxy-io/filament/connectors/object"
func init() {
	registry.RegisterSink("s3", func() filament.Sink { return New() })
}
