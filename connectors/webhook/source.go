// Package webhook consumes the managed webhook inbox as a continuous source.
package webhook

import (
	"context"
	"errors"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
	"github.com/galaxy-io/filament/webhooks"
	"github.com/galaxy-io/filament/webhooks/inbox"
	"github.com/galaxy-io/filament/webhooks/stream"
)

// PositionCodec identifies webhook inbox sequence positions.
const PositionCodec = inbox.PositionCodec

// Source reads accepted deliveries from the managed webhook inbox.
type Source struct {
	stream.Link
	batch stream.Batch
}

// New returns a source that attaches to the inbox the process supplies.
func New() *Source { return &Source{batch: stream.DefaultBatch()} }

// NewWithInbox uses an already open inbox, for local execution and tests.
func NewWithInbox(i *inbox.Inbox) *Source {
	s := New()
	s.SetWebhookInbox(stream.Shared(i), nil)
	return s
}

// Spec describes the webhook source connector and its stream capabilities.
func (*Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name: "webhook", DisplayName: "Webhook", Version: "1",
		Description:  "Receive signed webhook deliveries through Filament's durable inbox.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-webhook-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-webhook-light.svg",
		Stream: &filament.StreamCapabilities{
			Input:    filament.InputMessages,
			Ordering: []filament.Ordering{filament.OrderingNone},
			Delivery: filament.DeliveryReplayableAtLeastOnce,
		},
		Config:  filament.ConfigSchema{Fields: stream.BatchFields()},
		Webhook: &filament.WebhookSpec{Verifier: webhooks.GenericHMAC, DynamicResources: true},
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
	_, err = s.Inbox(ctx)
	return err
}

// TestConnection verifies that the source can attach to the inbox.
func (s *Source) TestConnection(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	return s.Probe(ctx)
}

// Extract rejects batch extraction because this source requires a continuous run.
func (*Source) Extract(context.Context, filament.RecordSink, filament.ExtractOpts) error {
	return filament.ErrContinuousDisabled
}

// Teardown releases the inbox connection owned by this source.
func (s *Source) Teardown(context.Context) error {
	s.Close()
	return nil
}

// PlanReplicationStream binds fixed resources to an inbox consumer.
func (*Source) PlanReplicationStream(req filament.ReplicationStreamPlanningRequest) (filament.ReplicationStreamPlan, error) {
	return stream.Plan(req)
}

// BindReplicationStream verifies the admitted consumer identity.
func (*Source) BindReplicationStream(config map[string]any, admitted filament.ReplicationStream) (map[string]any, error) {
	return stream.Bind(config, admitted)
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
