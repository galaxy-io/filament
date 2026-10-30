package aws

import (
	"context"
	"errors"
	"testing"

	sdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/galaxy-io/filament"
)

type fakeAPI struct {
	API
	put     *secretsmanager.PutSecretValueInput
	get     *secretsmanager.GetSecretValueInput
	create  *secretsmanager.CreateSecretInput
	missing bool
	raw     string
}

func (f *fakeAPI) PutSecretValue(_ context.Context, in *secretsmanager.PutSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.PutSecretValueOutput, error) {
	f.put = in
	if f.missing {
		return nil, &smtypes.ResourceNotFoundException{}
	}
	f.raw = sdk.ToString(in.SecretString)
	return &secretsmanager.PutSecretValueOutput{VersionId: in.ClientRequestToken}, nil
}

func (f *fakeAPI) CreateSecret(_ context.Context, in *secretsmanager.CreateSecretInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.CreateSecretOutput, error) {
	f.create = in
	f.raw = sdk.ToString(in.SecretString)
	return &secretsmanager.CreateSecretOutput{VersionId: in.ClientRequestToken}, nil
}

func (f *fakeAPI) GetSecretValue(_ context.Context, in *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	f.get = in
	return &secretsmanager.GetSecretValueOutput{SecretString: sdk.String(f.raw), VersionId: in.VersionId}, nil
}

func TestRotationKeepsSecretName(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing", true: "missing"}[missing], func(t *testing.T) {
			api := &fakeAPI{missing: missing}
			p := New(api, WithPrefix("company"))
			ref := filament.ConnectionSecretRef("tenant", "connection", "token", 1)
			err := p.RotateSecret(t.Context(), ref, filament.Secret{Tenant: "tenant", Value: []byte("rotated")})
			if missing {
				if !errors.Is(err, filament.ErrNotFound) {
					t.Fatalf("missing secret: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				value, err := p.Read(t.Context(), ref)
				if err != nil || string(value.Value) != "rotated" {
					t.Fatal("rotation not readable")
				}
				if api.get.VersionId != nil {
					t.Fatal("read pinned an old version")
				}
			}
			if api.create != nil {
				t.Fatal("rotation created a secret")
			}
			if sdk.ToString(api.put.SecretId) != "company/tenant/connection/token/v1" {
				t.Fatal("secret name changed")
			}
			if len(sdk.ToString(api.put.ClientRequestToken)) < 32 {
				t.Fatal("missing idempotency token")
			}
		})
	}
}

func TestWriteCanCreateSecret(t *testing.T) {
	api := &fakeAPI{missing: true}
	p := New(api)
	if err := p.Write(t.Context(), "ref", filament.Secret{Value: []byte("initial")}); err != nil {
		t.Fatal(err)
	}
	if api.create == nil || sdk.ToString(api.create.ClientRequestToken) != sdk.ToString(api.put.ClientRequestToken) {
		t.Fatal("create lost idempotency token")
	}
}
