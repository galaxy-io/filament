// Package object registers the supported object-store connectors.
package object

import (
	"github.com/galaxy-io/filament"
	objectgcs "github.com/galaxy-io/filament/connectors/object/gcs"
	objects3 "github.com/galaxy-io/filament/connectors/object/s3"
	"github.com/galaxy-io/filament/registry"
)

// init registers the supported object-store sinks on the default registry.
// Enable them with:
//
//	import _ "github.com/galaxy-io/filament/connectors/object"
func init() {
	registry.RegisterSink("gcs", true, func() filament.Sink { return objectgcs.New() })
	registry.RegisterSink("s3", true, func() filament.Sink { return objects3.New() })
}
