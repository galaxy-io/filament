package source

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament"
)

// Source reads subject resources using managed consumers, or binds explicit
// user-owned consumers for compatibility. One connection serves all resources.
type Source struct {
	conn     *nats.Conn
	js       nats.JetStreamContext
	identity string
	bindings []streamBinding
}

// New returns an unconfigured source.
func New() *Source { return &Source{} }

// Validate checks connection settings. Stream and consumer are pipeline settings.
func (*Source) Validate(cfg filament.Config) error {
	if cfg.String("url") == "" {
		return fmt.Errorf("nats: url is required")
	}
	return nil
}

// Configure opens one connection. TLS is supported through tls:// server URLs.
func (s *Source) Configure(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	var bindings []streamBinding
	if cfg.Has("streams") || cfg.Has("stream") || cfg.Has("consumer") {
		if cfg.Has("subjects") {
			return fmt.Errorf("nats: subject resources cannot be combined with explicit stream consumers")
		}
		var err error
		bindings, err = configuredStreams(cfg)
		if err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	opts := []nats.Option{nats.Timeout(5 * time.Second), nats.NoReconnect()}
	if file := cfg.String("credentials_file"); file != "" {
		opts = append(opts, nats.UserCredentials(file))
	}
	if token := cfg.Secret("token"); token != "" {
		opts = append(opts, nats.Token(token))
	}
	conn, err := nats.Connect(cfg.String("url"), opts...)
	if err != nil {
		return err
	}
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return err
	}
	s.conn = conn
	s.js = js
	// An empty binding set selects managed subject resources at OpenStream.
	s.bindings = bindings
	return nil
}

// Extract rejects bounded execution; this source requires the native stream loop.
func (*Source) Extract(context.Context, filament.RecordSink, filament.ExtractOpts) error {
	return filament.ErrContinuousDisabled
}

// Teardown releases the source connection after its session has closed.
func (s *Source) Teardown(context.Context) error {
	if s.conn != nil {
		s.conn.Close()
	}
	return nil
}

// TestConnection verifies JetStream access using a temporary connection.
func (s *Source) TestConnection(ctx context.Context, cfg filament.Config) (err error) {
	temp := New()
	if err := temp.Configure(ctx, cfg); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, temp.Teardown(ctx)) }()
	_, err = temp.js.AccountInfo(nats.Context(ctx))
	return err
}

var (
	_ filament.Source          = (*Source)(nil)
	_ filament.StreamSource    = (*Source)(nil)
	_ filament.SchemaProvider  = (*Source)(nil)
	_ filament.Discoverable    = (*Source)(nil)
	_ filament.LiveValidatable = (*Source)(nil)
)
