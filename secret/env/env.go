// Package env is an ingestion.Secrets provider backed by process environment
// variables. A reference like "postgres-creds/dsn" maps to the env var
// POSTGRES_CREDS_DSN. It is read-only and intended for local and development
// deployments.
package env

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	ingestion "github.com/galaxy-io/filament"
)

var _ ingestion.Secrets = (*Provider)(nil)

// ErrReadOnly is returned by writes: the process environment is fixed at runtime.
var ErrReadOnly = errors.New("env secret provider is read-only")

// Provider reads secrets from environment variables.
type Provider struct{}

// New returns an env-backed secret provider.
func New() *Provider { return &Provider{} }

// Read returns the value of the environment variable that ref maps to.
func (p *Provider) Read(ctx context.Context, ref string) (ingestion.Secret, error) {
	key := envKey(ref)
	v, ok := os.LookupEnv(key)
	if !ok {
		return ingestion.Secret{}, fmt.Errorf("env: %q (%s) not set: %w", ref, key, ingestion.ErrNotFound)
	}
	return ingestion.Secret{Value: []byte(v)}, nil
}

// Write is unsupported: the process environment is read-only at runtime.
func (p *Provider) Write(ctx context.Context, ref string, s ingestion.Secret) error {
	return ErrReadOnly
}

// Delete is unsupported: the process environment is read-only at runtime.
func (p *Provider) Delete(ctx context.Context, ref string) error { return ErrReadOnly }

// Name identifies the provider.
func (p *Provider) Name() string { return "env" }

// envKey converts a secret reference to an environment variable name:
// upper-cased, with '-', '/', and '.' replaced by '_'.
func envKey(ref string) string {
	return strings.ToUpper(strings.NewReplacer("-", "_", "/", "_", ".", "_").Replace(ref))
}
