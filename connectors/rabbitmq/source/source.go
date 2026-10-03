package source

import (
	"context"
	"errors"
	"slices"

	rmq "github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

// Source consumes replayable RabbitMQ Streams. Classic queue delivery tags are
// intentionally not used as durable positions because they are channel-local.
type Source struct {
	env *rmq.Environment
	uri string
	defaultStreams []string
}

// New returns an unconfigured RabbitMQ Streams source.
func New() *Source { return &Source{} }

func (*Source) Validate(cfg filament.Config) error {
	if cfg.String("uri") == "" {
		return errors.New("rabbitmq: uri is required")
	}
	return nil
}

func (s *Source) Configure(ctx context.Context, cfg filament.Config) error {
	if s.env != nil {
		return errors.New("rabbitmq: already configured")
	}
	if err := s.Validate(cfg); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	env, err := rmq.NewEnvironment(rmq.NewEnvironmentOptions().SetUri(cfg.String("uri")))
	if err != nil {
		return err
	}
	s.env, s.uri = env, cfg.String("uri")
	s.defaultStreams = configStrings(cfg, "streams")
	return nil
}

func (*Source) Extract(context.Context, filament.RecordSink, filament.ExtractOpts) error {
	return filament.ErrContinuousDisabled
}

func (s *Source) Teardown(context.Context) error {
	if s.env == nil {
		return nil
	}
	err := s.env.Close()
	s.env = nil
	return err
}

func (*Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name:        "rabbitmq",
		DisplayName: "RabbitMQ",
		Description: "Consume RabbitMQ Streams with Filament-owned durable offsets.",
		DarkLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-rabbitmq-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-rabbitmq-light.svg",
		Version:     "1",
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "uri", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "RabbitMQ Stream URI, for example rabbitmq-stream://user:password@host:5552/%2f"},
			{Name: "streams", Type: filament.FieldList, Scope: filament.ScopePipeline, Help: "RabbitMQ stream resources to consume"},
		}},
		Stream: &filament.StreamCapabilities{Input: filament.InputMessages, Ordering: []filament.Ordering{filament.OrderingNone}, Delivery: filament.DeliveryReplayableAtLeastOnce},
	}
}

func configStrings(cfg filament.Config, key string) []string {
	raw := cfg.Raw()[key]
	switch v := raw.(type) {
	case []string:
		return slices.Clone(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func messageBaseSchema(resource string) rowmodel.Schema {
	return rowmodel.Schema{Resource: resource, Fields: []rowmodel.Field{
		{Name: "stream", Logical: rowmodel.LogicalString},
		{Name: "offset", Logical: rowmodel.LogicalInt64},
	}}
}

var (
	_ filament.Source       = (*Source)(nil)
	_ filament.StreamSource = (*Source)(nil)
)
