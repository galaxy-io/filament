// Package env is a secret.Provider backed by process environment variables.
// A reference like "postgres-creds/dsn" maps to the env var POSTGRES_CREDS_DSN.
// It is read-only and intended for local and development deployments.
package env

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/secret"
)

var _ secret.Provider = (*Provider)(nil)

// Provider reads secrets from environment variables.
type Provider struct{}

// New returns an env-backed secret provider.
func New() *Provider { return &Provider{} }

// Read returns the value of the environment variable that ref maps to.
func (p *Provider) Read(ctx context.Context, ref string) (secret.Value, error) {
	key := envKey(ref)
	v, ok := os.LookupEnv(key)
	if !ok {
		return secret.Value{}, fmt.Errorf("env: %q (%s) not set: %w", ref, key, secret.ErrNotFound)
	}
	return secret.Value{Bytes: []byte(v)}, nil
}

// Write is unsupported: the process environment is read-only at runtime.
func (p *Provider) Write(ctx context.Context, ref string, v secret.Value) error {
	return secret.ErrReadOnly
}

// Delete is unsupported: the process environment is read-only at runtime.
func (p *Provider) Delete(ctx context.Context, ref string) error {
	return secret.ErrReadOnly
}

// Name identifies the provider.
func (p *Provider) Name() string { return "env" }

var _ ingestion.Secrets = (*IngestionAdapter)(nil)

// IngestionAdapter wraps a Provider to satisfy ingestion.Secrets.
type IngestionAdapter struct{ *Provider }

// NewIngestionSecrets returns an env-backed ingestion.Secrets implementation.
func NewIngestionSecrets() *IngestionAdapter { return &IngestionAdapter{Provider: New()} }

func (a *IngestionAdapter) Read(ctx context.Context, ref string) (ingestion.Secret, error) {
	v, err := a.Provider.Read(ctx, ref)
	if err != nil {
		return ingestion.Secret{}, err
	}
	return ingestion.Secret{Value: v.Bytes, Meta: v.Meta}, nil
}

func (a *IngestionAdapter) Write(ctx context.Context, ref string, s ingestion.Secret) error {
	return a.Provider.Write(ctx, ref, secret.Value{Bytes: s.Value, Meta: s.Meta})
}

// envKey converts a secret reference to an environment variable name:
// upper-cased, with '-', '/', and '.' replaced by '_'.
func envKey(ref string) string {
	return strings.ToUpper(strings.NewReplacer("-", "_", "/", "_", ".", "_").Replace(ref))
}
