package sample

import (
	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

// init registers the sample source on the default registry. Enable with:
//
//	import _ "github.com/galaxy-io/filament/connectors/sample"
func init() {
	registry.RegisterSource("sample", func() ingestion.Source { return New() })
}
