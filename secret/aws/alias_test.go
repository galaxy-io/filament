package aws

import (
	"context"
	"errors"
	"strings"
	"testing"

	sdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/galaxy-io/filament"
)

// namedAPI stores secrets by name. arns maps an ARN to the name it resolves
// to, as Secrets Manager does for a SecretId.
type namedAPI struct {
	API
	secrets map[string]string
	arns    map[string]string
	deleted []string
}

func (f *namedAPI) resolve(id string) string {
	if name, ok := f.arns[id]; ok {
		return name
	}
	return id
}

func (f *namedAPI) PutSecretValue(_ context.Context, in *secretsmanager.PutSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
	name := f.resolve(sdk.ToString(in.SecretId))
	if _, ok := f.secrets[name]; !ok {
		return nil, &smtypes.ResourceNotFoundException{}
	}
	f.secrets[name] = sdk.ToString(in.SecretString)
	return &secretsmanager.PutSecretValueOutput{}, nil
}

func (f *namedAPI) CreateSecret(_ context.Context, in *secretsmanager.CreateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	f.secrets[sdk.ToString(in.Name)] = sdk.ToString(in.SecretString)
	return &secretsmanager.CreateSecretOutput{}, nil
}

func (f *namedAPI) GetSecretValue(_ context.Context, in *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	raw, ok := f.secrets[f.resolve(sdk.ToString(in.SecretId))]
	if !ok {
		return nil, &smtypes.ResourceNotFoundException{}
	}
	return &secretsmanager.GetSecretValueOutput{SecretString: sdk.String(raw)}, nil
}

func (f *namedAPI) DeleteSecret(_ context.Context, in *secretsmanager.DeleteSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.DeleteSecretOutput, error) {
	f.deleted = append(f.deleted, f.resolve(sdk.ToString(in.SecretId)))
	return &secretsmanager.DeleteSecretOutput{}, nil
}

// A secret Filament saved for a connection may only be reached through the
// ref it was saved under, however else Secrets Manager could resolve its name.
func TestManagedSecretsAreOnlyReachableByTheirRef(t *testing.T) {
	ref := filament.ConnectionSecretRef("tenant", "conn", "password", 1) + "/attempt"
	// What the server stores alongside every connection secret.
	saved := filament.Secret{Tenant: "tenant", Value: []byte("saved"), Meta: map[string]string{"tenant": "tenant", "connection": "conn", "field": "password"}}

	cases := []struct {
		name   string
		prefix string
		// alias returns another ref for the same secret, given the
		// secret's Secrets Manager name.
		alias func(api *namedAPI, name string) string
	}{
		{
			name:   "prefix set, ref without the filament segment",
			prefix: "acme",
			alias:  func(*namedAPI, string) string { return strings.TrimPrefix(ref, filament.ConnectionSecretPrefix) },
		},
		{
			name: "no prefix, ARN",
			alias: func(api *namedAPI, name string) string {
				arn := "arn:aws:secretsmanager:us-east-1:123456789012:secret:" + name
				api.arns[arn] = name
				return arn
			},
		},
		// Under a prefix an ARN-shaped ref becomes "acme/arn:...", which
		// Secrets Manager treats as a plain name, so it cannot alias.
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := &namedAPI{secrets: map[string]string{}, arns: map[string]string{}}
			p := New(api, WithPrefix(tc.prefix))
			if err := p.Write(t.Context(), ref, saved); err != nil {
				t.Fatal(err)
			}
			name := p.name(ref)
			alias := tc.alias(api, name)
			// Guard against a vacuous pass: the alias must really reach the
			// saved secret.
			if got := api.resolve(p.name(alias)); got != name {
				t.Fatalf("alias %q resolves to %q, not the saved secret %q", alias, got, name)
			}

			if got, err := p.Read(t.Context(), ref); err != nil || string(got.Value) != "saved" {
				t.Fatalf("read by its own ref = %q, %v", got.Value, err)
			}
			if got, err := p.Read(t.Context(), alias); err == nil {
				t.Errorf("read by alias %q returned %q", alias, got.Value)
			}
			if err := p.Delete(t.Context(), alias); !errors.Is(err, ErrUnmanaged) {
				t.Errorf("delete by alias %q: err = %v, want ErrUnmanaged", alias, err)
			}
			if len(api.deleted) != 0 {
				t.Errorf("delete by alias removed %v", api.deleted)
			}
		})
	}
}

// Secrets an operator stores for env-style refs carry no tenant and stay
// readable by the ref that names them.
func TestUnmanagedSecretsStayReadable(t *testing.T) {
	for _, prefix := range []string{"", "acme"} {
		t.Run("prefix "+prefix, func(t *testing.T) {
			api := &namedAPI{secrets: map[string]string{}, arns: map[string]string{}}
			p := New(api, WithPrefix(prefix))
			if err := p.Write(t.Context(), "PG_PASSWORD", filament.Secret{Value: []byte("operator")}); err != nil {
				t.Fatal(err)
			}
			if got, err := p.Read(t.Context(), "PG_PASSWORD"); err != nil || string(got.Value) != "operator" {
				t.Fatalf("read = %q, %v", got.Value, err)
			}
		})
	}
}
