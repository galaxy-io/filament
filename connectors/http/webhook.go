package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/response"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
	"github.com/galaxy-io/filament/webhooks/inbox"
	"github.com/galaxy-io/filament/webhooks/stream"
)

// A manifest with a webhook block makes its source a continuous source as
// well. Resources with mode: webhook are read from the managed inbox, which
// the composition root supplies through stream.Sources. The session, resume
// and coverage belong to webhooks/stream. This file only decodes and projects
// delivery bodies the way an API response would be.

var (
	_ filament.StreamSource             = (*Source)(nil)
	_ filament.ReplicationStreamPlanner = (*Source)(nil)
	_ stream.InboxReader                = (*Source)(nil)
)

// errNoWebhooks reports a stream request against a source whose manifest
// receives no webhooks.
var errNoWebhooks = errors.New("httpapi source: manifest declares no webhooks")

// webhookManifest is the embedded manifest when it declares webhooks. The
// generic manifest-path source never receives webhooks: its manifest is
// unknown until configuration, and the spec must be known before.
func (s *Source) webhookManifest() *manifest.Manifest {
	if s.embeddedManifest == nil || s.embeddedManifest.Webhook == nil {
		return nil
	}
	return s.embeddedManifest
}

// webhookSpec is the source's webhook spec, nil when it receives none.
func webhookSpec(m *manifest.Manifest) *filament.WebhookSpec {
	if m == nil || m.Webhook == nil {
		return nil
	}
	// Validation built the verifier from these params, so they encode.
	params, _ := m.Webhook.Verifier.ParamsJSON()
	var resources []string
	for _, r := range m.Resources {
		if r.IsWebhook() {
			resources = append(resources, r.Name)
		}
	}
	return &filament.WebhookSpec{
		Verifier: m.Webhook.Verifier.Profile, VerifierParams: params, Ingress: m.IngressJSON(),
		IngestionField: manifest.IngestionField, Setup: m.Webhook.Setup, Resources: resources,
		SecretIssuer: m.Webhook.Secret,
	}
}

// webhookStreamCapabilities describes webhook resources: typed rows with
// declared keys, replayed from the inbox until certified, in no order.
func webhookStreamCapabilities(m *manifest.Manifest) *filament.StreamCapabilities {
	if m == nil || m.Webhook == nil {
		return nil
	}
	return &filament.StreamCapabilities{
		Input:    filament.InputRows,
		Ordering: []filament.Ordering{filament.OrderingNone},
		Delivery: filament.DeliveryReplayableAtLeastOnce,
	}
}

func ingestionField(m *manifest.Manifest) filament.ConfigField {
	return filament.ConfigField{
		Name: manifest.IngestionField, Type: filament.FieldEnum, Default: filament.IngestionAPI, Scope: filament.ScopeConnection,
		Enum: []filament.EnumOption{
			{Value: filament.IngestionAPI, Label: "API"},
			{Value: filament.IngestionWebhooks, Label: "Webhooks"},
			{Value: filament.IngestionBoth, Label: "API and webhooks"},
		},
		Help: fmt.Sprintf("How this connection reads %s. Webhooks needs no API credentials. With both, webhook resources stream continuously next to the scheduled API resources.", m.DisplayName),
	}
}

// apiVisibleWhen shows a manifest config field only to connections that read
// the API.
var apiVisibleWhen = filament.FieldCondition{
	Field: manifest.IngestionField, Values: []string{filament.IngestionAPI, filament.IngestionBoth},
}

// ingestion returns how a connection with cfg reads the source. A source
// without webhooks reads only the API.
func (s *Source) ingestion(cfg filament.Config) string {
	if s.webhookManifest() == nil {
		return filament.IngestionAPI
	}
	return filament.IngestionOf(cfg.String(manifest.IngestionField))
}

// webhookResources lists the manifest's webhook resources for discovery.
func (s *Source) webhookResources(ctx context.Context, m *manifest.Manifest) []filament.Resource {
	var out []filament.Resource
	for _, r := range m.Resources {
		if !r.IsWebhook() {
			continue
		}
		schema, _ := s.Schema(ctx, r.Name)
		metadata := map[string]string{filament.ResourceMetaIngress: filament.ResourceIngressWebhook}
		out = append(out, filament.Resource{
			Name: r.Name, Selector: r.Name, Selectable: true,
			PrimaryKey: append([]string(nil), r.PrimaryKey...), Schema: &schema, Metadata: metadata,
		})
	}
	return out
}

// PlanReplicationStream binds webhook resources to an inbox consumer. A
// continuous run reads only webhook resources, and only for a connection
// that turned webhook ingestion on.
func (s *Source) PlanReplicationStream(req filament.ReplicationStreamPlanningRequest) (filament.ReplicationStreamPlan, error) {
	m := s.webhookManifest()
	if m == nil {
		return filament.ReplicationStreamPlan{}, errNoWebhooks
	}
	if req.Config == nil || s.ingestion(req.Config) == filament.IngestionAPI {
		return filament.ReplicationStreamPlan{}, fmt.Errorf("%s source: this connection reads only the API", s.name)
	}
	for _, name := range req.Resources {
		if r, ok := findResource(m.Resources, name); !ok || !r.IsWebhook() {
			return filament.ReplicationStreamPlan{}, fmt.Errorf("%s source: %q is not a webhook resource and runs bounded", s.name, name)
		}
	}
	return stream.Plan(req)
}

// BindReplicationStream verifies the admitted consumer identity.
func (s *Source) BindReplicationStream(config map[string]any, admitted filament.ReplicationStream) (map[string]any, error) {
	if s.webhookManifest() == nil {
		return nil, errNoWebhooks
	}
	return stream.Bind(config, admitted)
}

// Lookup resolves the inbox sequence codec.
func (s *Source) Lookup(name string, version int) (rowmodel.PositionCodec, error) {
	if s.webhookManifest() == nil || name != inbox.PositionCodec || version != 0 {
		return nil, fmt.Errorf("%s source: unknown position codec %s v%d", s.name, name, version)
	}
	return streamkit.Uint64Codec{}, nil
}

// OpenStream reads the connection's deliveries and writes each one as rows
// of its webhook resource.
func (s *Source) OpenStream(ctx context.Context, opts filament.StreamOpenOpts) (filament.StreamSession, error) {
	if s.webhookManifest() == nil {
		return nil, errNoWebhooks
	}
	if s.connector == nil || s.connector.manifest == nil {
		return nil, fmt.Errorf("%s source: open stream before configure", s.name)
	}
	if opts.MessageSchemaPolicy != "" {
		return nil, fmt.Errorf("%s source: webhook resources have declared schemas and take no message schema policy", s.name)
	}
	i, err := s.Inbox(ctx)
	if err != nil {
		return nil, err
	}
	rows := newRowSink(ctx, s, nil)
	return stream.Open(ctx, stream.Config{
		Inbox: i, Batch: stream.DefaultBatch(), Codecs: s,
		Write: func(ctx context.Context, out filament.StreamRecordSink, batch []stream.Delivery) error {
			rows.sink = out
			return s.writeDeliveries(ctx, rows, batch)
		},
	}, opts)
}

// writeDeliveries decodes each delivery body with its resource's records path
// and projects every record with the resource's fields. A body that does not
// decode is rejected and skipped, so one malformed delivery never stalls the
// stream. Failing to write a decoded row still fails the session.
func (s *Source) writeDeliveries(ctx context.Context, rows *rowSink, batch []stream.Delivery) error {
	for _, d := range batch {
		res, ok := s.manifestResource(d.Resource)
		if !ok || !res.IsWebhook() {
			s.Reject(ctx, d, fmt.Errorf("names unknown webhook resource %q", d.Resource))
			continue
		}
		records, err := decodeDelivery(rows, res, d.Body, s.connector.creds)
		if err != nil {
			s.Reject(ctx, d, err)
			continue
		}
		for n, out := range records {
			identity := d.Identity.Clone()
			// Records of one delivery share its position and keep their order.
			identity.Ordinal = uint64(n) //nolint:gosec // n is a non-negative slice index
			out.Stream = &rowmodel.StreamMeta{Identity: identity}
			if err := rows.Push(out); err != nil {
				return err
			}
		}
	}
	return nil
}

// decodeDelivery turns one delivery body into its resource's records and
// checks each against the resource schema. Every record decodes before any is
// written, so a delivery is written whole or rejected whole.
func decodeDelivery(rows *rowSink, res manifest.Resource, body []byte, creds map[string]string) ([]record, error) {
	raw, err := response.New(res.Response).Records(body)
	if err != nil {
		return nil, err
	}
	out := make([]record, 0, len(raw))
	for _, rec := range raw {
		data, projected, err := projectRecord(res, rec, nil, creds)
		if err != nil {
			return nil, err
		}
		keyJSON, err := json.Marshal(extractKey(data, res.PrimaryKey))
		if err != nil {
			return nil, err
		}
		dataJSON, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		rec := newHTTPRecord(res.Name, keyJSON, dataJSON, projected)
		if err := rows.Check(rec); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}
