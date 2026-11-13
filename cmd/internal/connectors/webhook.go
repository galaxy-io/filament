package connectors

import (
	"context"
	"os"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/webhook"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/webhooks/inbox"
)

// The webhook source reads the inbox the server provisions. The process reads
// NATS_URL here, so the connector itself takes no environment.
func init() {
	registry.RegisterSource("webhook", true, func() filament.Source {
		return webhook.New(func(ctx context.Context) (*inbox.Inbox, func(), error) {
			return inbox.Connect(ctx, os.Getenv("NATS_URL"), "filament-webhook-source")
		})
	})
}
