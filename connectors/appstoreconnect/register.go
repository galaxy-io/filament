// Package appstoreconnect wires the Apple App Store Connect source onto the default registry.
// Enable it with a blank import:
//
//	import _ "github.com/galaxy-io/filament/connectors/appstoreconnect"
package appstoreconnect

import (
	"github.com/galaxy-io/filament"
	source "github.com/galaxy-io/filament/connectors/appstoreconnect/source"
	"github.com/galaxy-io/filament/registry"
)

func init() {
	registry.RegisterSource("appstoreconnect", true, func() filament.Source { return source.New() })
}
