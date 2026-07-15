// Package aws implements ingestion.Secrets backed by AWS Secrets Manager.
//
// A secret reference maps directly to a Secrets Manager secret name. Both the
// plaintext value and its metadata are preserved by storing a JSON envelope in
// the secret's SecretString field, so Read/Write/Delete round-trip the full
// ingestion.Secret the way the env and postgres providers do.
package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	ingestion "github.com/galaxy-io/filament"
)

var _ ingestion.Secrets = (*Provider)(nil)

// API is the subset of the Secrets Manager client the provider uses. It is
// satisfied by *secretsmanager.Client and is exported so callers can inject a
// fake in tests.
type API interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
	CreateSecret(ctx context.Context, params *secretsmanager.CreateSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error)
	PutSecretValue(ctx context.Context, params *secretsmanager.PutSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error)
	DeleteSecret(ctx context.Context, params *secretsmanager.DeleteSecretInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error)
}

// Provider stores secrets in AWS Secrets Manager.
type Provider struct {
	client API
	prefix string
}

// Option configures a Provider.
type Option func(*Provider)

// WithPrefix namespaces every ref under the given Secrets Manager name prefix,
// e.g. "filament/" turns the ref "tenant-a/pg-dsn" into the secret name
// "filament/tenant-a/pg-dsn".
func WithPrefix(prefix string) Option { return func(p *Provider) { p.prefix = prefix } }

// New constructs a provider from an explicit Secrets Manager API. Use this to
// inject a preconfigured client (custom region, credentials, or a test fake).
func New(client API, opts ...Option) *Provider {
	p := &Provider{client: client}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// NewFromConfig loads the default AWS config chain (env, shared config,
// IAM role) and returns a Secrets Manager–backed provider.
func NewFromConfig(ctx context.Context, opts ...Option) (*Provider, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("secret/aws: load config: %w", err)
	}
	return New(secretsmanager.NewFromConfig(cfg), opts...), nil
}

// Name returns the provider identifier.
func (p *Provider) Name() string { return "aws" }

// envelope is the JSON stored as the secret's SecretString so both value and
// metadata survive a round trip.
type envelope struct {
	Value []byte            `json:"value"`
	Meta  map[string]string `json:"meta,omitempty"`
}

func (p *Provider) name(ref string) string { return p.prefix + ref }

// Read fetches and decodes the secret at ref.
func (p *Provider) Read(ctx context.Context, ref string) (ingestion.Secret, error) {
	name := p.name(ref)
	out, err := p.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{SecretId: aws.String(name)})
	if err != nil {
		var notFound *smtypes.ResourceNotFoundException
		if errors.As(err, &notFound) {
			return ingestion.Secret{}, fmt.Errorf("secret/aws: read %q: %w", ref, ingestion.ErrNotFound)
		}
		return ingestion.Secret{}, fmt.Errorf("secret/aws: read %q: %w", ref, err)
	}
	var raw []byte
	switch {
	case out.SecretString != nil:
		raw = []byte(*out.SecretString)
	case out.SecretBinary != nil:
		raw = out.SecretBinary
	default:
		return ingestion.Secret{}, fmt.Errorf("secret/aws: read %q: empty secret", ref)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return ingestion.Secret{}, fmt.Errorf("secret/aws: read %q: decode: %w", ref, err)
	}
	return ingestion.Secret{Value: env.Value, Meta: env.Meta}, nil
}

// Write creates the secret if absent, otherwise stores a new version.
func (p *Provider) Write(ctx context.Context, ref string, s ingestion.Secret) error {
	name := p.name(ref)
	raw, err := json.Marshal(envelope{Value: s.Value, Meta: s.Meta})
	if err != nil {
		return fmt.Errorf("secret/aws: write %q: encode: %w", ref, err)
	}
	body := string(raw)
	_, err = p.client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(name),
		SecretString: aws.String(body),
	})
	if err == nil {
		return nil
	}
	var notFound *smtypes.ResourceNotFoundException
	if !errors.As(err, &notFound) {
		return fmt.Errorf("secret/aws: write %q: %w", ref, err)
	}
	// Secret does not exist yet; create it.
	if _, err := p.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(name),
		SecretString: aws.String(body),
	}); err != nil {
		return fmt.Errorf("secret/aws: create %q: %w", ref, err)
	}
	return nil
}

// ErrUnmanaged is returned by Delete when the provider has no configured
// prefix, so it cannot prove a secret is one Filament created.
var ErrUnmanaged = errors.New("secret/aws: refusing to delete without a managed prefix")

// Delete schedules the secret at ref for deletion using Secrets Manager's
// default recovery window; deleting a missing ref is a no-op. To avoid
// destroying secrets Filament did not create, Delete only operates when a
// non-empty prefix is configured (see WithPrefix); otherwise it returns
// ErrUnmanaged.
func (p *Provider) Delete(ctx context.Context, ref string) error {
	if p.prefix == "" {
		return fmt.Errorf("delete %q: %w", ref, ErrUnmanaged)
	}
	_, err := p.client.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId: aws.String(p.name(ref)),
	})
	if err != nil {
		var notFound *smtypes.ResourceNotFoundException
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("secret/aws: delete %q: %w", ref, err)
	}
	return nil
}
