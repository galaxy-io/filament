package connectors

import (
	"context"
	"os"

	"github.com/galaxy-io/filament"
	_ "github.com/galaxy-io/filament/connectors/webhook" // registers the generic webhook source
	"github.com/galaxy-io/filament/telemetry"
	"github.com/galaxy-io/filament/webhooks/inbox"
	"github.com/galaxy-io/filament/webhooks/stream"
)

// withWebhookInbox gives every source that reads the webhook inbox an
// attacher to the inbox the server provisions. The process reads NATS_URL
// here, so connectors take no environment.
func withWebhookInbox(sources filament.SourceRegistry, metrics *telemetry.Set) filament.SourceRegistry {
	return stream.Sources{SourceRegistry: sources, Metrics: metrics, Attach: func(ctx context.Context) (*inbox.Inbox, func(), error) {
		return inbox.Connect(ctx, os.Getenv("NATS_URL"), "filament-webhook-source")
	}}
}
