package httpapi

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

// init registers the manifest-driven HTTP source plus the bundled SaaS catalog
// (Notion, Linear). Enable with:
//
//	import _ "github.com/galaxy-io/filament/connectors/http"
//
// New SaaS connectors are a catalog/<name>.yaml manifest + one line here — no
// new module, no new dependency.
func init() {
	registry.RegisterSource("httpapi", func() ingestion.Source { return New() })
	registry.RegisterSource("notion", func() ingestion.Source { return NewNotion() })
	registry.RegisterSource("linear", func() ingestion.Source { return NewLinear() })
}
