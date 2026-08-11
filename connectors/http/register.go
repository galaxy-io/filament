package httpapi

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

// init registers the bundled manifest-driven SaaS catalog
// (Notion, Linear, Attio, GitHub, Slack, Resend). Enable with:
//
//	import _ "github.com/galaxy-io/filament/connectors/http"
//
// New SaaS connectors are a catalog/<name>.yaml manifest + one line here — no
// new module, no new dependency.
func init() {
	registry.RegisterSource("notion", func() filament.Source { return NewNotion() })
	registry.RegisterSource("linear", func() filament.Source { return NewLinear() })
	registry.RegisterSource("attio", func() filament.Source { return NewAttio() })
	registry.RegisterSource("github", func() filament.Source { return NewGitHub() })
	registry.RegisterSource("slack", func() filament.Source { return NewSlack() })
	registry.RegisterSource("resend", func() filament.Source { return NewResend() })
}
