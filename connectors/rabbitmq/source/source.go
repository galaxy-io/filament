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
	if cfg.String("host") == "" {
		return errors.New("rabbitmq: host is required")
	}
	if cfg.String("username") == "" {
		return errors.New("rabbitmq: username is required")
	}
	if cfg.Secret("password") == "" {
		return errors.New("rabbitmq: password is required")
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

func (*Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name:        "rabbitmq",
		DisplayName: "RabbitMQ Streams",
		Description: "Consume RabbitMQ Streams with durable offset-based replay.",
		Version:     "1",
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "host", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "RabbitMQ Stream host"},
			{Name: "port", Type: filament.FieldInt, Default: 5552, Scope: filament.ScopeConnection, Help: "RabbitMQ Stream protocol port"},
			{Name: "username", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection},
			{Name: "password", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection},
			{Name: "streams", Type: filament.FieldList, Scope: filament.ScopePipeline, Help: "RabbitMQ stream resources to consume"},
		}},
		Stream: &filament.StreamCapabilities{
			Input: filament.InputMessages,
			Ordering: []filament.Ordering{filament.OrderingNone},
			Delivery: filament.DeliveryReplayableAtLeastOnce,
		},
	}
}

var (
	_ filament.Source       = (*Source)(nil)
	_ filament.StreamSource = (*Source)(nil)
)
