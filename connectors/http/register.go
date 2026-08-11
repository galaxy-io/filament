package httpapi

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

// init registers the bundled manifest-driven SaaS catalog.
// Enable with:
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
	registry.RegisterSource("square", func() filament.Source { return NewSquare() })
	registry.RegisterSource("stripe", func() filament.Source { return NewStripe() })
	registry.RegisterSource("posthog", func() filament.Source { return NewPostHog() })
	registry.RegisterSource("chargebee", func() filament.Source { return NewChargebee() })
}
