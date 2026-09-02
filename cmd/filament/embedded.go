package main

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"sync"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/app"
	sqlitestore "github.com/galaxy-io/filament/datastore/sqlite"
	sqlitemetrics "github.com/galaxy-io/filament/datastore/sqlite/metrics"
	secretenv "github.com/galaxy-io/filament/secret/env"
)

// dbPath is the durable local deployment state, next to the config documents:
// the YAML owns configuration, the database owns the operational record (runs,
// checkpoints, versions). Nothing ever wipes it; delete the file to reset.
func (a *cliApp) dbPath() string {
	dir := filepath.Dir(a.contextPath)
	if a.contextPath == "" {
		// Harness-built apps (tests) have no context registry; keep the
		// database beside the document rather than the working directory.
		dir = filepath.Dir(a.configPath)
	}
	return filepath.Join(dir, "filament.db")
}

// embeddedOptions composes the embedded deployment's stores: the SQLite
// datastore and metrics store over one handle, and session secrets over env.
func (a *cliApp) embeddedOptions() ([]app.Option, *embeddedSecrets, error) {
	store, err := sqlitestore.Open(a.dbPath())
	if err != nil {
		return nil, nil, err
	}
	secrets := newEmbeddedSecrets()
	return []app.Option{
		app.WithDataStore(store),
		app.WithMetricsStore(sqlitemetrics.New(store.DB())),
		app.WithSecrets(secrets),
	}, secrets, nil
}

// startEmbedded boots the full filament stack — inproc bus, SQLite store,
// engine, tracker, orchestrator, scheduler, and the ConnectRPC API — on a
// loopback listener inside this process. It returns the endpoint to dial,
// the deployment's secrets provider, and a stop that tears the stack down.
func (a *cliApp) startEmbedded(ctx context.Context) (string, filament.Secrets, func(), error) {
	options, secrets, err := a.embeddedOptions()
	if err != nil {
		return "", nil, nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, nil, fmt.Errorf("embedded listener: %w", err)
	}
	serveCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = app.Serve(serveCtx, listener, options...)
	}()
	stop := func() {
		cancel()
		<-done
	}
	return "http://" + listener.Addr().String(), secrets, stop, nil
}

// embeddedSecrets accepts writes into process memory and reads through to
// environment variables. The embedded deployment must be able to store the
// plaintext secret values a local YAML may carry; they live exactly as long
// as the deployment, and the YAML round-trip restores them on the next boot.
// env:NAME references resolve from the environment.
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
