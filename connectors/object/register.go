// Package object registers the supported object-store connectors.
package object

import (
	"github.com/galaxy-io/filament"
	objects3 "github.com/galaxy-io/filament/connectors/object/s3"
	"github.com/galaxy-io/filament/registry"
)

// init registers the s3 object-store sink on the default registry. Enable with:
//
//	import _ "github.com/galaxy-io/filament/connectors/object"
func init() {
	registry.RegisterSink("s3", filament.MaturityAlpha, func() filament.Sink { return objects3.New() })
}
