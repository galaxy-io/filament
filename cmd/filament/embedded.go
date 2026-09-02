package main

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/app"
	secretenv "github.com/galaxy-io/filament/secret/env"
)

// startEmbedded boots the full filament stack — inproc bus, memory store,
// engine, tracker, orchestrator, scheduler, and the ConnectRPC API — on a
// loopback listener inside this process. It returns the endpoint to dial and
// a stop that tears the stack down.
func startEmbedded(ctx context.Context) (string, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("embedded listener: %w", err)
	}
	serveCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = app.Serve(serveCtx, listener, app.WithSecrets(newEmbeddedSecrets()))
	}()
	stop := func() {
		cancel()
		<-done
	}
	return "http://" + listener.Addr().String(), stop, nil
}

// embeddedSecrets accepts writes into process memory and reads through to
// environment variables. The embedded deployment must be able to store the
// plaintext secret values a local YAML may carry; they live exactly as long
// as the deployment does. env:NAME references resolve from the environment.
type embeddedSecrets struct {
	mu     sync.RWMutex
	values map[string]filament.Secret
	env    filament.Secrets
}

func newEmbeddedSecrets() *embeddedSecrets {
	return &embeddedSecrets{values: map[string]filament.Secret{}, env: secretenv.New()}
}

func (s *embeddedSecrets) Name() string { return "embedded" }

func (s *embeddedSecrets) Read(ctx context.Context, ref string) (filament.Secret, error) {
	s.mu.RLock()
	secret, ok := s.values[ref]
	s.mu.RUnlock()
	if ok {
		return secret, nil
	}
	return s.env.Read(ctx, ref)
}

func (s *embeddedSecrets) Write(_ context.Context, ref string, secret filament.Secret) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[ref] = secret
	return nil
}

func (s *embeddedSecrets) Delete(_ context.Context, ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, ref)
	return nil
}
