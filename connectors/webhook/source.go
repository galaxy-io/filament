// Package webhook consumes the managed webhook inbox as a continuous source.
package webhook

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
	"github.com/galaxy-io/filament/webhooks"
	"github.com/galaxy-io/filament/webhooks/inbox"
	"github.com/galaxy-io/filament/webhooks/stream"
)

// PositionCodec identifies webhook inbox sequence positions.
const PositionCodec = inbox.PositionCodec

// Attacher connects to the managed inbox. The returned close releases the
// connection. The receiver provisions the stream, so an attacher never creates it.
type Attacher func(context.Context) (*inbox.Inbox, func(), error)

// Source reads accepted deliveries from the managed webhook inbox.
type Source struct {
	attach Attacher
	inbox  *inbox.Inbox
	close  func()
	batch  stream.Batch
}

// New returns a source that attaches to the inbox when configured.
func New(attach Attacher) *Source { return &Source{attach: attach, batch: stream.DefaultBatch()} }

// NewWithInbox uses an already open inbox, for local execution and tests.
func NewWithInbox(i *inbox.Inbox) *Source { return &Source{inbox: i, batch: stream.DefaultBatch()} }

// Spec describes the webhook source connector and its stream capabilities.
func (*Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name: "webhook", DisplayName: "Webhook", Version: "1",
		Description:     "Receive signed webhook deliveries through Filament's durable inbox.",
		WebhookVerifier: webhooks.GenericHMAC,
		Stream: &filament.StreamCapabilities{
			Input:    filament.InputMessages,
			Ordering: []filament.Ordering{filament.OrderingNone},
			Delivery: filament.DeliveryReplayableAtLeastOnce,
		},
		Config: filament.ConfigSchema{Fields: stream.BatchFields()},
	}
}

// Validate checks the source's batch configuration.
func (*Source) Validate(cfg filament.Config) error {
	_, err := stream.ParseBatch(cfg)
	return err
}

// Configure parses batch settings and attaches to the inbox.
func (s *Source) Configure(ctx context.Context, cfg filament.Config) error {
	batch, err := stream.ParseBatch(cfg)
	if err != nil {
		return err
	}
	s.batch = batch
	if s.inbox != nil {
		return nil
	}
	if s.attach == nil {
		return errors.New("webhook: no inbox is configured for this process")
	}
	s.inbox, s.close, err = s.attach(ctx)
	return err
}

// TestConnection verifies that the source can attach to the inbox.
func (s *Source) TestConnection(ctx context.Context, cfg filament.Config) error {
	temp := New(s.attach)
	if err := temp.Configure(ctx, cfg); err != nil {
		return err
	}
	return temp.Teardown(ctx)
}

// Extract rejects batch extraction because this source requires a continuous run.
func (*Source) Extract(context.Context, filament.RecordSink, filament.ExtractOpts) error {
	return filament.ErrContinuousDisabled
}

// Teardown releases the inbox connection owned by this source.
func (s *Source) Teardown(context.Context) error {
	if s.close != nil {
		s.close()
		s.close = nil
		s.inbox = nil
	}
	return nil
}

// PlanReplicationStream binds fixed resources to an inbox consumer.
func (*Source) PlanReplicationStream(req filament.ReplicationStreamPlanningRequest) (filament.ReplicationStreamPlan, error) {
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
		ConsumerName: "webhook-" + req.ReplicationStreamID,
		ContinuityConfig: map[string]any{
			"source_connection_id": req.SourceConnectionID,
			"inbox_stream":         inbox.DefaultStream,
		},
	}, nil
}

// BindReplicationStream verifies the admitted consumer identity.
func (*Source) BindReplicationStream(config map[string]any, admitted filament.ReplicationStream) (map[string]any, error) {
	if admitted.ID == "" || admitted.SourceConnectionID == "" ||
		admitted.ConsumerName != "webhook-"+admitted.ID {
		return nil, errors.New("webhook: admitted consumer identity mismatch")
	}
	return maps.Clone(config), nil
}

// Lookup returns the position codec used by webhook inbox sequences.
func (*Source) Lookup(name string, version int) (rowmodel.PositionCodec, error) {
	if name != PositionCodec || version != 0 {
		return nil, fmt.Errorf("webhook: unknown position codec %s v%d", name, version)
	}
	return streamkit.Uint64Codec{}, nil
}

// Schema returns the delivery envelope schema for a resource.
func (*Source) Schema(_ context.Context, resource string) (rowmodel.Schema, error) {
	if !webhooks.ValidResourceName(resource) {
		return rowmodel.Schema{}, errors.New("webhook: invalid resource")
	}
	return envelopeSchema(resource), nil
}

func envelopeSchema(resource string) rowmodel.Schema {
	return rowmodel.Schema{Resource: resource, Fields: []rowmodel.Field{
		{Name: "delivery_id", Logical: rowmodel.LogicalString},
		{Name: "provider_delivery_id", Logical: rowmodel.LogicalString},
		{Name: "received_at", Logical: rowmodel.LogicalTimestampTZ},
		{Name: "payload", Logical: rowmodel.LogicalBytes},
		{Name: "metadata", Logical: rowmodel.LogicalBytes},
	}}
}

// messageBaseSchema keeps delivery identity separate from the source-owned
// JSON fields discovered from accepted webhook bodies.
func messageBaseSchema(resource string) rowmodel.Schema {
	return rowmodel.Schema{Resource: resource, Fields: []rowmodel.Field{
		{Name: "delivery_id", Logical: rowmodel.LogicalString},
		{Name: "provider_delivery_id", Logical: rowmodel.LogicalString},
		{Name: "received_at", Logical: rowmodel.LogicalTimestampTZ},
		{Name: "metadata", Logical: rowmodel.LogicalBytes},
		{Name: "tenant_id", Logical: rowmodel.LogicalString},
		{Name: "source_connection_id", Logical: rowmodel.LogicalString},
	}}
}

var _ filament.MessageSchemaSource = (*Source)(nil)

var (
	_ filament.Source                   = (*Source)(nil)
	_ filament.StreamSource             = (*Source)(nil)
	_ filament.ReplicationStreamPlanner = (*Source)(nil)
	_ filament.SchemaProvider           = (*Source)(nil)
	_ filament.LiveValidatable          = (*Source)(nil)
)
