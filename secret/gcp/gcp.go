// Package gcp implements filament.Secrets backed by Google Cloud Secret Manager.
package gcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/galaxy-io/filament"
)

const (
	managedLabel  = "managed-by"
	managedValue  = "filament"
	refAnnotation = "filament-ref"
	namePrefix    = "filament-"
	maxSecretID   = 255
)

var (
	_             filament.Secrets = (*Provider)(nil)
	prefixPattern                  = regexp.MustCompile(`^[A-Za-z0-9_-]*$`)
)

// API is the subset of the Secret Manager client used by Provider.
type API interface {
	AccessSecretVersion(context.Context, *secretmanagerpb.AccessSecretVersionRequest, ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error)
	GetSecret(context.Context, *secretmanagerpb.GetSecretRequest, ...gax.CallOption) (*secretmanagerpb.Secret, error)
	CreateSecret(context.Context, *secretmanagerpb.CreateSecretRequest, ...gax.CallOption) (*secretmanagerpb.Secret, error)
	AddSecretVersion(context.Context, *secretmanagerpb.AddSecretVersionRequest, ...gax.CallOption) (*secretmanagerpb.SecretVersion, error)
	DeleteSecret(context.Context, *secretmanagerpb.DeleteSecretRequest, ...gax.CallOption) error
}

// Provider stores secrets in Google Cloud Secret Manager.
type Provider struct {
	client    API
	projectID string
	region    string
	prefix    string
	close     func() error
}

// Option configures a Provider.
type Option func(*Provider)

// WithPrefix prepends a deployment-specific prefix to each GCP secret ID.
func WithPrefix(prefix string) Option { return func(p *Provider) { p.prefix = prefix } }

// New constructs a provider from an explicit Secret Manager API.
func New(client API, projectID, region string, opts ...Option) (*Provider, error) {
	p := &Provider{client: client, projectID: projectID, region: region}
	for _, opt := range opts {
		opt(p)
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("secret/gcp: project ID is required (set GCP_PROJECT_ID)")
	}
	if strings.TrimSpace(region) == "" {
		return nil, errors.New("secret/gcp: region is required (set GCP_REGION)")
	}
	if strings.Contains(region, "/") {
		return nil, errors.New("secret/gcp: GCP_REGION must be a location ID")
	}
	if !prefixPattern.MatchString(p.prefix) {
		return nil, errors.New("secret/gcp: SECRETS_PREFIX may contain only letters, digits, hyphens, and underscores")
	}
	if len(p.prefix)+len(namePrefix)+sha256.Size*2 > maxSecretID {
		return nil, fmt.Errorf("secret/gcp: SECRETS_PREFIX is too long (maximum %d characters)", maxSecretID-len(namePrefix)-sha256.Size*2)
	}
	return p, nil
}

// NewFromConfig uses Application Default Credentials to construct a provider.
func NewFromConfig(ctx context.Context, projectID, region string, opts ...Option) (*Provider, error) {
	p, err := New(nil, projectID, region, opts...)
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("secretmanager.%s.rep.googleapis.com:443", region)
	client, err := secretmanager.NewClient(ctx, option.WithEndpoint(endpoint))
	if err != nil {
		return nil, fmt.Errorf("secret/gcp: create client: %w", err)
	}
	p.client = client
	p.close = client.Close
	return p, nil
}

// Name returns the provider identifier.
func (p *Provider) Name() string { return "gcp-secret-manager" }

// Close releases the underlying Google Cloud client when Provider created it.
func (p *Provider) Close() error {
	if p.close == nil {
		return nil
	}
	return p.close()
}

type envelope struct {
	Ref    string            `json:"ref"`
	Tenant filament.TenantID `json:"tenant,omitempty"`
	Value  []byte            `json:"value"`
	Meta   map[string]string `json:"meta,omitempty"`
}

func (p *Provider) secretID(ref string) string {
	sum := sha256.Sum256([]byte(ref))
	return p.prefix + namePrefix + hex.EncodeToString(sum[:])
}

func (p *Provider) secretName(ref string) string {
	return fmt.Sprintf("%s/secrets/%s", p.parentName(), p.secretID(ref))
}

func (p *Provider) parentName() string {
	return fmt.Sprintf("projects/%s/locations/%s", p.projectID, p.region)
}

// Read fetches and decodes the latest version of the secret at ref.
func (p *Provider) Read(ctx context.Context, ref string) (filament.Secret, error) {
	out, err := p.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{Name: p.secretName(ref) + "/versions/latest"})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return filament.Secret{}, fmt.Errorf("secret/gcp: read %q: %w", ref, filament.ErrNotFound)
		}
		return filament.Secret{}, fmt.Errorf("secret/gcp: read %q: %w", ref, err)
	}
	if out.Payload == nil {
		return filament.Secret{}, fmt.Errorf("secret/gcp: read %q: empty secret", ref)
	}
	var env envelope
	if err := json.Unmarshal(out.Payload.Data, &env); err != nil {
		return filament.Secret{}, fmt.Errorf("secret/gcp: read %q: decode: %w", ref, err)
	}
	if env.Ref != ref || env.Value == nil {
		return filament.Secret{}, fmt.Errorf("secret/gcp: read %q: not a matching filament envelope", ref)
	}
	return filament.Secret{Tenant: env.Tenant, Value: env.Value, Meta: env.Meta}, nil
}

// Write creates the secret if absent, otherwise stores a new version.
func (p *Provider) Write(ctx context.Context, ref string, s filament.Secret) error {
	raw, err := json.Marshal(envelope{Ref: ref, Tenant: s.Tenant, Value: s.Value, Meta: s.Meta})
	if err != nil {
		return fmt.Errorf("secret/gcp: write %q: encode: %w", ref, err)
	}
	name := p.secretName(ref)
	secret, err := p.client.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{Name: name})
	if status.Code(err) == codes.NotFound {
		secret, err = p.client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
			Parent:   p.parentName(),
			SecretId: p.secretID(ref),
			Secret: &secretmanagerpb.Secret{
				Labels:      map[string]string{managedLabel: managedValue},
				Annotations: map[string]string{refAnnotation: ref},
			},
		})
		if status.Code(err) == codes.AlreadyExists {
			secret, err = p.client.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{Name: name})
		}
	}
	if err != nil {
		return fmt.Errorf("secret/gcp: prepare %q: %w", ref, err)
	}
	if err := managed(secret, ref); err != nil {
		return fmt.Errorf("secret/gcp: write %q: %w", ref, err)
	}
	_, err = p.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: name, Payload: &secretmanagerpb.SecretPayload{Data: raw},
	})
	if err != nil {
		return fmt.Errorf("secret/gcp: write %q: %w", ref, err)
	}
	return nil
}

// ErrUnmanaged is returned when Provider cannot prove it owns a secret.
var ErrUnmanaged = errors.New("secret/gcp: refusing to modify an unmanaged secret")

func managed(secret *secretmanagerpb.Secret, ref string) error {
	if secret == nil || secret.Labels[managedLabel] != managedValue || secret.Annotations[refAnnotation] != ref {
		return ErrUnmanaged
	}
	return nil
}

// Delete permanently deletes the secret at ref. Deleting a missing ref is a
// no-op. GCP metadata is checked before its irreversible deletion.
func (p *Provider) Delete(ctx context.Context, ref string) error {
	if p.prefix == "" && !strings.HasPrefix(ref, filament.ConnectionSecretPrefix) {
		return fmt.Errorf("delete %q: %w", ref, ErrUnmanaged)
	}
	name := p.secretName(ref)
	secret, err := p.client.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{Name: name})
	if status.Code(err) == codes.NotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("secret/gcp: delete %q: inspect: %w", ref, err)
	}
	if err := managed(secret, ref); err != nil {
		return fmt.Errorf("secret/gcp: delete %q: %w", ref, err)
	}
	if err := p.client.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{Name: name}); err != nil {
		if status.Code(err) == codes.NotFound {
			return nil
		}
		return fmt.Errorf("secret/gcp: delete %q: %w", ref, err)
	}
	return nil
}
