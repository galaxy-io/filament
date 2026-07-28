package httpapi

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

// init registers the manifest-driven HTTP source plus the bundled SaaS catalog
// (Notion, Linear, GitHub, Slack). Enable with:
//
//	import _ "github.com/galaxy-io/filament/connectors/http"
//
// New SaaS connectors are a catalog/<name>.yaml manifest + one line here — no
// new module, no new dependency.
func init() {
	registry.RegisterSource("httpapi", func() filament.Source { return New() })
	registry.RegisterSource("notion", func() filament.Source { return NewNotion() })
	registry.RegisterSource("linear", func() filament.Source { return NewLinear() })
	registry.RegisterSource("github", func() filament.Source { return NewGitHub() })
	registry.RegisterSource("slack", func() filament.Source { return NewSlack() })
}
