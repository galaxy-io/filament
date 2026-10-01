// Package source consumes Kafka partitions using Filament-owned checkpoints.
package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/kafka/internal/client"
	"github.com/galaxy-io/filament/rowmodel"
)

// Source uses direct partition assignment. It never joins a Kafka consumer group
// or mutates broker offsets; Filament certificates own durable progress.
type Source struct {
	client  *kgo.Client
	options []kgo.Opt
	start   string
}

// New returns a fresh connector instance.
func New() *Source { return &Source{} }

var (
	_ filament.Source                   = (*Source)(nil)
	_ filament.StreamSource             = (*Source)(nil)
	_ filament.Discoverable             = (*Source)(nil)
	_ filament.SchemaProvider           = (*Source)(nil)
	_ filament.ReplicationStreamPlanner = (*Source)(nil)
	_ filament.LiveValidatable          = (*Source)(nil)
)

// Validate checks configuration without contacting the broker.
func (*Source) Validate(cfg filament.Config) error {
	if err := client.Validate(cfg); err != nil {
		return err
	}
	if s := cfg.String("start_position"); s != "" && s != "earliest" && s != "latest" {
		return errors.New("kafka: start_position must be earliest or latest")
	}
	if cfg.Has("topics") {
		ts, err := client.Strings(cfg, "topics")
		if err != nil {
			return err
		}
		_, err = topics(ts)
		return err
	}
	return nil
}

// Configure creates the metadata client and saves options for stream sessions.
func (s *Source) Configure(ctx context.Context, cfg filament.Config) error {
	if s.client != nil {
		return errors.New("kafka: already configured")
	}
	if err := s.Validate(cfg); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	opts, err := client.Options(cfg)
	if err != nil {
		return err
	}
	c, err := kgo.NewClient(opts...)
	if err != nil {
		return err
	}
	s.client, s.options, s.start = c, opts, cfg.String("start_position")
	return nil
}

// Extract rejects bounded execution; this source uses the continuous stream loop.
func (*Source) Extract(context.Context, filament.RecordSink, filament.ExtractOpts) error {
	return filament.ErrContinuousDisabled
}

// Teardown releases the metadata client after stream sessions have closed.
func (s *Source) Teardown(context.Context) error {
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}
	return nil
}

// TestConnection checks authentication and metadata access without producing records.
func (*Source) TestConnection(ctx context.Context, cfg filament.Config) error {
	return client.TestConnection(ctx, cfg)
}

// Spec describes configuration and supported execution capabilities.
func (*Source) Spec() filament.ConnectorSpec {
	fields := append(client.Fields(), filament.ConfigField{Name: "topics", Type: filament.FieldList, Scope: filament.ScopePipeline, Help: "Default topic resources"}, filament.ConfigField{Name: "start_position", Type: filament.FieldString, Default: "earliest", Scope: filament.ScopePipeline, Help: "earliest or latest for partitions without certified progress"})
	return filament.ConnectorSpec{Name: "kafka", DisplayName: client.DisplayName, DarkLogoURL: client.DarkLogoURL, LightLogoURL: client.LightLogoURL, Description: "Read Kafka message envelopes with Filament-owned partition checkpoints.", Version: "1", Config: filament.ConfigSchema{Fields: fields}, Stream: &filament.StreamCapabilities{Input: filament.InputMessages, Ordering: []filament.Ordering{filament.OrderingNone}, Delivery: filament.DeliveryReplayableAtLeastOnce}}
}

func messageBaseSchema(resource string) rowmodel.Schema {
	return rowmodel.Schema{Resource: resource, Fields: []rowmodel.Field{{Name: "topic", Logical: rowmodel.LogicalString}, {Name: "partition", Logical: rowmodel.LogicalInt32}, {Name: "offset", Logical: rowmodel.LogicalInt64}, {Name: "tombstone", Logical: rowmodel.LogicalBool}}}
}

// Schema returns transport and event metadata. Payload columns are inferred on the first read.
func Schema(resource string) (rowmodel.Schema, error) {
	return rowmodel.WithEventMetadataFields(messageBaseSchema(resource))
}

// Schema returns transport and event metadata. Payload columns are inferred on the first read.
func (*Source) Schema(_ context.Context, resource string) (rowmodel.Schema, error) {
	if err := client.ValidateTopic(resource); err != nil {
		return rowmodel.Schema{}, err
	}
	return Schema(resource)
}

// Discover lists accessible noninternal topics.
func (s *Source) Discover(ctx context.Context, _ filament.DiscoverOpts) (filament.DiscoverResult, error) {
	if s.client == nil {
		return filament.DiscoverResult{}, errors.New("kafka: source is not configured")
	}
	m, err := client.Metadata(ctx, s.client, nil)
	if err != nil {
		return filament.DiscoverResult{}, err
	}
	var rs []filament.Resource
	for _, t := range m.Topics {
		if !t.IsInternal {
			rs = append(rs, filament.Resource{Name: *t.Topic, Selectable: true})
		}
	}
	slices.SortFunc(rs, func(a, b filament.Resource) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return filament.DiscoverResult{Resources: rs}, nil
}

func topics(in []string) ([]string, error) {
	out := slices.Clone(in)
	slices.Sort(out)
	if len(out) == 0 {
		return nil, errors.New("kafka: select at least one topic")
	}
	for i, t := range out {
		if err := client.ValidateTopic(t); err != nil {
			return nil, err
		}
		if i > 0 && out[i-1] == t {
			return nil, fmt.Errorf("kafka: duplicate topic %q", t)
		}
	}
	return out, nil
}

// PlanReplicationStream binds stable topic resources to Filament-owned checkpoints.
func (s *Source) PlanReplicationStream(req filament.ReplicationStreamPlanningRequest) (filament.ReplicationStreamPlan, error) {
	if err := s.Validate(req.Config); err != nil {
		return filament.ReplicationStreamPlan{}, err
	}
	if req.SourceConnectionID == "" || req.ReplicationStreamID == "" {
		return filament.ReplicationStreamPlan{}, errors.New("kafka: source and replication stream identities required")
	}
	selected := req.Resources
	if len(selected) == 0 {
		var err error
		selected, err = client.Strings(req.Config, "topics")
		if err != nil {
			return filament.ReplicationStreamPlan{}, err
		}
	}
	selected, err := topics(selected)
	if err != nil {
		return filament.ReplicationStreamPlan{}, err
	}
	start := req.Config.String("start_position")
	if start == "" {
		start = "earliest"
	}
	config := map[string]any{"topics": selected, "start_position": start, "assignment": "direct"}
	return filament.ReplicationStreamPlan{Resources: selected, ConsumerName: "filament-" + req.ReplicationStreamID, ConsumerConfig: config, ContinuityConfig: map[string]any{"source_identity": req.SourceConnectionID, "topics": selected, "start_position": start, "assignment": "direct"}}, nil
}

// BindReplicationStream applies the admitted direct-assignment configuration.
func (*Source) BindReplicationStream(config map[string]any, admitted filament.ReplicationStream) (map[string]any, error) {
	if admitted.ID == "" || admitted.ConsumerName != "filament-"+admitted.ID || admitted.ConsumerConfig["assignment"] != "direct" {
		return nil, errors.New("kafka: admitted identity mismatch")
	}
	raw, err := json.Marshal(admitted.ConsumerConfig["topics"])
	if err != nil {
		return nil, err
	}
	var selected []string
	if err := json.Unmarshal(raw, &selected); err != nil {
		return nil, err
	}
	if _, err = topics(selected); err != nil {
		return nil, err
	}
	start, ok := admitted.ConsumerConfig["start_position"].(string)
	if !ok || (start != "earliest" && start != "latest") {
		return nil, errors.New("kafka: invalid admitted start position")
	}
	out := maps.Clone(config)
	out["topics"], out["start_position"] = selected, start
	return out, nil
}
