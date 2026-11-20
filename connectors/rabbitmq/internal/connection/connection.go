// Package connection holds the RabbitMQ Streams URI settings shared by the sink
// and the source in https://github.com/galaxy-io/filament/pull/477.
package connection

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"

	"github.com/galaxy-io/filament"
)

// Catalog identity shared with the RabbitMQ Streams source.
const (
	DisplayName  = "RabbitMQ"
	DarkLogoURL  = "https://cdn.getgalaxy.io/sources/source-icon-rabbitmq-dark.svg"
	LightLogoURL = "https://cdn.getgalaxy.io/sources/source-icon-rabbitmq-light.svg"
)

// Fields returns the connection-scoped settings. The URI is a secret because it
// carries the password. Pipeline fields are declared by the sink.
func Fields() []filament.ConfigField {
	return []filament.ConfigField{{
		Name: "uri", Type: filament.FieldSecret, Secret: true, Required: true, Scope: filament.ScopeConnection,
		Help: "RabbitMQ Stream URI, for example rabbitmq-stream://user:password@host:5552/%2f",
	}}
}

// Validate checks the Stream URI. Schemes match the source connector.
func Validate(cfg filament.Config) error {
	raw := cfg.String("uri")
	if raw == "" {
		raw = cfg.Secret("uri")
	}
	if raw == "" {
		return errors.New("rabbitmq: uri is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("rabbitmq: invalid uri: %w", err)
	}
	if u.Scheme != "rabbitmq-stream" && u.Scheme != "rabbitmq-stream+tls" {
		return fmt.Errorf("rabbitmq: unsupported uri scheme %q", u.Scheme)
	}
	if u.Hostname() == "" {
		return errors.New("rabbitmq: uri must include a hostname")
	}
	if u.Fragment != "" {
		return errors.New("rabbitmq: uri must not include a fragment")
	}
	return nil
}

// URI returns the configured Stream URI.
func URI(cfg filament.Config) string {
	if raw := cfg.String("uri"); raw != "" {
		return raw
	}
	return cfg.Secret("uri")
}

// Dial opens a Streams environment. The caller closes it.
func Dial(ctx context.Context, raw string) (*stream.Environment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	env, err := stream.NewEnvironment(stream.NewEnvironmentOptions().SetUri(raw))
	if err != nil {
		if env != nil {
			_ = env.Close()
		}
		return nil, err
	}
	return env, nil
}
