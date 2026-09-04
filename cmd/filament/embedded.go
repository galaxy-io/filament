package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/app"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/datastore/sqlite/metrics"
	secretenv "github.com/galaxy-io/filament/secret/env"
	secretsqlite "github.com/galaxy-io/filament/secret/sqlite"
)

// stateDir holds the local deployment's durable state beside the context
// registry: filament.db, secret.key, and the applied-document marker.
// Harness-built apps have no registry and keep it beside the document.
func (a *cliApp) stateDir() string {
	if a.contextPath == "" {
		return filepath.Dir(a.configPath)
	}
	return filepath.Dir(a.contextPath)
}

func (a *cliApp) dbPath() string     { return filepath.Join(a.stateDir(), "filament.db") }
func (a *cliApp) keyPath() string    { return filepath.Join(a.stateDir(), "secret.key") }
func (a *cliApp) markerPath() string { return filepath.Join(a.stateDir(), "filament.db.applied") }

// embeddedOptions opens the deployment's stores: the sqlite datastore and
// metrics over one handle, and secrets in the same database with env:NAME
// references read through to the environment.
func (a *cliApp) embeddedOptions() ([]app.Option, filament.Secrets, error) {
	store, err := sqlite.Open(a.dbPath())
	if err != nil {
		return nil, nil, err
	}
	key, err := a.secretKey()
	if err != nil {
		return nil, nil, err
	}
	stored, err := secretsqlite.New(store.DB(), "local", key)
	if err != nil {
		return nil, nil, err
	}
	secrets := layeredSecrets{stored: stored, env: secretenv.New()}
	return []app.Option{
		app.WithDataStore(store),
		app.WithMetricsStore(metrics.New(store.DB())),
		app.WithSecrets(secrets),
	}, secrets, nil
}

// secretKey returns the key that encrypts stored secrets, minting one on
// first use. Losing the file loses the stored secrets, not the rest.
func (a *cliApp) secretKey() ([]byte, error) {
	encoded, err := os.ReadFile(a.keyPath())
	if err == nil {
		return base64.StdEncoding.DecodeString(string(encoded))
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(a.stateDir(), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(a.keyPath(), []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

// startEmbedded boots the full stack on a loopback listener inside this
// process and returns the endpoint to dial, the secrets provider, and a stop.
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

// layeredSecrets stores in sqlite and reads through to the environment, so
// a document may carry plaintext values or env:NAME references.
type layeredSecrets struct {
	stored *secretsqlite.Provider
	env    filament.Secrets
}

func (s layeredSecrets) Name() string { return "local" }

func (s layeredSecrets) Read(ctx context.Context, ref string) (filament.Secret, error) {
	secret, err := s.stored.Read(ctx, ref)
	if errors.Is(err, filament.ErrNotFound) {
		return s.env.Read(ctx, ref)
	}
	return secret, err
}

func (s layeredSecrets) Write(ctx context.Context, ref string, secret filament.Secret) error {
	return s.stored.Write(ctx, ref, secret)
}

func (s layeredSecrets) Delete(ctx context.Context, ref string) error {
	return s.stored.Delete(ctx, ref)
}
