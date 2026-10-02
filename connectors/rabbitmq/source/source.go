package source

import (
	"context"
	"errors"

	"github.com/galaxy-io/filament"
)

// Source consumes replayable RabbitMQ Streams. Classic queue delivery tags are
// intentionally not used as durable positions because they are channel-local.
type Source struct{}

// New returns an unconfigured RabbitMQ Streams source.
func New() *Source { return &Source{} }

func (*Source) Validate(cfg filament.Config) error {
	if cfg.String("uri") == "" {
		return errors.New("rabbitmq: uri is required")
	}
	return nil
}

func (s *Source) Configure(_ context.Context, cfg filament.Config) error {
	return s.Validate(cfg)
}

func (*Source) Extract(context.Context, filament.RecordSink, filament.ExtractOpts) error {
	return filament.ErrContinuousDisabled
}

func (*Source) Teardown(context.Context) error { return nil }

// OpenStream is wired in the streaming session implementation. Keeping the
// method on Source makes the replayable contract explicit at registration time.
func (*Source) OpenStream(context.Context, filament.StreamOpenOpts) (filament.StreamSession, error) {
	return nil, errors.New("rabbitmq: stream session not initialized")
}

func (*Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name:        "rabbitmq",
		DisplayName: "RabbitMQ Streams",
		Description: "Consume RabbitMQ Streams with durable offset-based replay.",
		Version:     "1",
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "uri", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "RabbitMQ Stream URI, for example rabbitmq-stream://user:password@host:5552/%2f"},
			{Name: "streams", Type: filament.FieldList, Scope: filament.ScopePipeline, Help: "RabbitMQ stream resources to consume"},
		}},
		Stream: &filament.StreamCapabilities{Input: filament.InputMessages, Ordering: []filament.Ordering{filament.OrderingNone}, Delivery: filament.DeliveryReplayableAtLeastOnce},
	}
}

var (
	_ filament.Source       = (*Source)(nil)
	_ filament.StreamSource = (*Source)(nil)
)
