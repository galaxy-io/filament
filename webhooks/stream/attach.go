package stream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/streamkit"
	"github.com/galaxy-io/filament/webhooks"
	"github.com/galaxy-io/filament/webhooks/inbox"
)

// Every source that reads the inbox shares its position codec.
func init() {
	registry.RegisterPositionCodec(inbox.PositionCodec, 0, streamkit.Uint64Codec{})
}

// ErrNoInbox reports that the process supplied no inbox to a source that
// reads one.
var ErrNoInbox = errors.New("webhook: no inbox is configured for this process")

// Attacher connects to the managed inbox. The returned close releases the
// connection. The receiver provisions the stream, so an attacher never creates it.
type Attacher func(context.Context) (*inbox.Inbox, func(), error)

// Shared returns an attacher for an inbox the process already holds open.
// Closing an attachment leaves the inbox open.
func Shared(i *inbox.Inbox) Attacher {
	return func(context.Context) (*inbox.Inbox, func(), error) { return i, func() {}, nil }
}

// InboxReader is a source that reads the managed inbox. The composition root
// supplies the attacher and the process metrics, so a connector never reads
// process configuration.
type InboxReader interface {
	SetWebhookInbox(Attacher, filament.Metrics)
}

// Sources gives Attach and Metrics to every resolved source that reads the
// inbox. Metrics may be nil.
type Sources struct {
	filament.SourceRegistry
	Attach  Attacher
	Metrics filament.Metrics
}

// Resolve returns a fresh source, attached when it reads the inbox.
func (s Sources) Resolve(name string) (filament.Source, error) {
	source, err := s.SourceRegistry.Resolve(name)
	if err != nil {
		return nil, err
	}
	if reader, ok := source.(InboxReader); ok && s.Attach != nil {
		reader.SetWebhookInbox(s.Attach, s.Metrics)
	}
	return source, nil
}

// Link is a source's attachment to the inbox. Embed it to implement
// InboxReader. It attaches on first use and Close releases the attachment.
type Link struct {
	attach  Attacher
	metrics filament.Metrics
	inbox   *inbox.Inbox
	close   func()
}

// SetWebhookInbox sets how the source attaches and where it counts rejected
// deliveries. It implements InboxReader.
func (l *Link) SetWebhookInbox(a Attacher, m filament.Metrics) { l.attach, l.metrics = a, m }

// Reject records a delivery the connector cannot turn into rows. The session
// skips it: its sequence still counts toward coverage, so one malformed body
// never stalls the stream. The delivery stays in the inbox until retention
// purges it, for inspection by its ID.
func (l *Link) Reject(ctx context.Context, d Delivery, err error) {
	slog.WarnContext(ctx, "webhook delivery rejected: body does not decode as its resource",
		"event.name", "webhook.delivery.rejected",
		"source_connection_id", d.Endpoint, "resource", d.Resource,
		"delivery_id", d.ID, "error", err)
	if l.metrics != nil {
		l.metrics.Counter("filament_webhook_rejected_deliveries_total",
			filament.Label{Key: "resource", Value: d.Resource}).Inc()
	}
}

// Inbox attaches once and returns the inbox.
func (l *Link) Inbox(ctx context.Context) (*inbox.Inbox, error) {
	if l.inbox != nil {
		return l.inbox, nil
	}
	if l.attach == nil {
		return nil, ErrNoInbox
	}
	i, closeFn, err := l.attach(ctx)
	if err != nil {
		return nil, err
	}
	l.inbox, l.close = i, closeFn
	return i, nil
}

// Probe checks that the source can attach, and releases an attachment it
// made for the check.
func (l *Link) Probe(ctx context.Context) error {
	if l.inbox != nil {
		return nil
	}
	if _, err := l.Inbox(ctx); err != nil {
		return err
	}
	l.Close()
	return nil
}

// Close releases the attachment. The next Inbox attaches again.
func (l *Link) Close() {
	if l.close != nil {
		l.close()
	}
	l.inbox, l.close = nil, nil
}

// consumerPrefix names every inbox consumer, so a source that changes
// connector keeps its admitted consumer identity.
const consumerPrefix = "webhook-"

// Plan binds fixed resources to the inbox consumer of the stream. Every
// resource must be safe in a delivery URL and named once.
func Plan(req filament.ReplicationStreamPlanningRequest) (filament.ReplicationStreamPlan, error) {
	if req.SourceConnectionID == "" || req.ReplicationStreamID == "" || len(req.Resources) == 0 {
		return filament.ReplicationStreamPlan{}, errors.New("webhook: source, stream, and fixed resources are required")
	}
	resources := slices.Clone(req.Resources)
	slices.Sort(resources)
	for n, resource := range resources {
		if !webhooks.ValidResourceName(resource) || (n > 0 && resources[n-1] == resource) {
			return filament.ReplicationStreamPlan{}, fmt.Errorf("webhook: invalid or repeated resource %q", resource)
		}
	}
	return filament.ReplicationStreamPlan{
		Resources:    resources,
		ConsumerName: consumerPrefix + req.ReplicationStreamID,
		ContinuityConfig: map[string]any{
			"source_connection_id": req.SourceConnectionID,
			"inbox_stream":         inbox.DefaultStream,
		},
	}, nil
}

// Bind verifies the admitted consumer identity of a planned stream.
func Bind(config map[string]any, admitted filament.ReplicationStream) (map[string]any, error) {
	if admitted.ID == "" || admitted.SourceConnectionID == "" ||
		admitted.ConsumerName != consumerPrefix+admitted.ID {
		return nil, errors.New("webhook: admitted consumer identity mismatch")
	}
	return maps.Clone(config), nil
}
