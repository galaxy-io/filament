// Package connection holds the connection settings, validation, and dialing
// shared by the NATS JetStream source and sink.
package connection

import (
	"context"
	"errors"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament"
)

// Catalog identity shared by both connectors.
const (
	DisplayName  = "NATS JetStream"
	DarkLogoURL  = "https://cdn.getgalaxy.io/sources/source-icon-nats-dark.svg"
	LightLogoURL = "https://cdn.getgalaxy.io/sources/source-icon-nats-light.svg"
)

const dialTimeout = 5 * time.Second

// Fields returns the connection-scoped configuration shared by the source and sink.
func Fields() []filament.ConfigField {
	return []filament.ConfigField{
		{Name: "url", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "NATS server URL; use tls:// for TLS"},
		{Name: "token", Type: filament.FieldSecret, Scope: filament.ScopeConnection, Help: "Authentication token; use this or credentials_file"},
		{Name: "credentials_file", Type: filament.FieldString, Scope: filament.ScopeConnection, Help: "Credentials file available on every worker; use this or token"},
	}
}

// Validate checks connection settings independently of pipeline settings.
func Validate(cfg filament.Config) error {
	if cfg.String("url") == "" {
		return errors.New("nats: url is required")
	}
	if cfg.Secret("token") != "" && cfg.String("credentials_file") != "" {
		return errors.New("nats: use token or credentials_file, not both")
	}
	return nil
}

// Connect dials one connection without reconnection. The dial timeout never
// outlives the context deadline, and cancellation before return closes it.
func Connect(ctx context.Context, cfg filament.Config) (*nats.Conn, error) {
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	timeout := dialTimeout
	if deadline, ok := ctx.Deadline(); ok {
		timeout = min(timeout, time.Until(deadline))
	}
	opts := []nats.Option{nats.Timeout(timeout), nats.NoReconnect()}
	if file := cfg.String("credentials_file"); file != "" {
		opts = append(opts, nats.UserCredentials(file))
	}
	if token := cfg.Secret("token"); token != "" {
		opts = append(opts, nats.Token(token))
	}
	conn, err := nats.Connect(cfg.String("url"), opts...)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}
