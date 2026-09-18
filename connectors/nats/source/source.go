// Package source implements consumption of dedicated JetStream consumers.
package source

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament"
)

// Source reads subject resources using managed consumers, or binds explicit
// user-owned consumers for compatibility. One connection serves all resources.
type Source struct {
	conn                       *nats.Conn
	js                         nats.JetStreamContext
	stream, consumer, identity string
	bindings                   []streamBinding
	resource                   string
	filters                    []string
	managed                    bool
}

// New returns an unconfigured source.
func New() *Source { return &Source{} }

// Validate checks connection settings. Stream and consumer are pipeline settings.
func (*Source) Validate(cfg filament.Config) error {
	for _, name := range []string{"url", "source_identity"} {
		if cfg.String(name) == "" {
			return fmt.Errorf("nats: %s is required", name)
		}
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
	s.stream = cfg.String("stream")
	s.consumer = cfg.String("consumer")
	s.identity = cfg.String("source_identity")
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
