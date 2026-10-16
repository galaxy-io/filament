package gcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"

	"github.com/galaxy-io/filament"
)

type fakeAPI struct {
	API
	ref        string
	read       *secretmanagerpb.AccessSecretVersionRequest
	add        *secretmanagerpb.AddSecretVersionRequest
	destroy    *secretmanagerpb.DestroySecretVersionRequest
	addErr     error
	destroyErr error
	calls      []string
}

func (f *fakeAPI) GetSecret(_ context.Context, in *secretmanagerpb.GetSecretRequest, _ ...gax.CallOption) (*secretmanagerpb.Secret, error) {
	return &secretmanagerpb.Secret{Name: in.Name, Labels: map[string]string{managedLabel: managedValue}, Annotations: map[string]string{refAnnotation: f.ref}}, nil
}

func (f *fakeAPI) AddSecretVersion(_ context.Context, in *secretmanagerpb.AddSecretVersionRequest, _ ...gax.CallOption) (*secretmanagerpb.SecretVersion, error) {
	f.calls = append(f.calls, "add")
	if f.addErr != nil {
		return nil, f.addErr
	}
	f.add = in
	return &secretmanagerpb.SecretVersion{Name: in.Parent + "/versions/7"}, nil
}

func (f *fakeAPI) AccessSecretVersion(_ context.Context, in *secretmanagerpb.AccessSecretVersionRequest, _ ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	f.calls = append(f.calls, "read")
	f.read = in
	if f.add == nil {
		return &secretmanagerpb.AccessSecretVersionResponse{Name: strings.TrimSuffix(in.Name, "latest") + "6", Payload: &secretmanagerpb.SecretPayload{Data: []byte(`{"ref":"` + f.ref + `","tenant":"tenant","value":"b2xk"}`)}}, nil
	}
	return &secretmanagerpb.AccessSecretVersionResponse{Name: strings.Replace(f.add.Parent, "projects/project/", "projects/123456/", 1) + "/versions/7", Payload: f.add.Payload}, nil
}

func TestImmutableVersionsKeepSecretName(t *testing.T) {
	ref := filament.ConnectionSecretRef("tenant", "connection", "token", 1)
	api := &fakeAPI{ref: ref}
	p, err := New(api, "project", "us-central1")
	if err != nil {
		t.Fatal(err)
	}
	version, err := p.writeVersion(t.Context(), ref, filament.Secret{Tenant: "tenant", Value: []byte("rotated"), Meta: map[string]string{"oauth_format": "v1"}})
	if err != nil || version != "7" {
		t.Fatalf("write version: %s %v", version, err)
	}
	if api.add.Parent != p.secretName(ref) {
		t.Fatal("secret identity changed")
	}
	value, got, err := p.readVersion(t.Context(), ref, version)
	if err != nil || got != version || value.Tenant != "tenant" || string(value.Value) != "rotated" || value.Meta["oauth_format"] != "v1" {
		t.Fatalf("roundtrip: %v", err)
	}
	if api.read.Name != p.secretName(ref)+"/versions/7" {
		t.Fatal("read latest instead of pinned version")
	}
	for _, invalid := range []string{"latest", "other/versions/7", "-1"} {
		if _, _, err := p.readVersion(t.Context(), ref, invalid); err == nil {
			t.Fatalf("accepted invalid version %s", invalid)
		}
	}
}

func (f *fakeAPI) DestroySecretVersion(_ context.Context, in *secretmanagerpb.DestroySecretVersionRequest, _ ...gax.CallOption) (*secretmanagerpb.SecretVersion, error) {
	f.calls = append(f.calls, "destroy")
	f.destroy = in
	return &secretmanagerpb.SecretVersion{Name: in.Name}, f.destroyErr
}

func TestRotationDestroysPreviousVersion(t *testing.T) {
	ref := filament.ConnectionSecretRef("tenant", "connection", "token", 1)
	api := &fakeAPI{ref: ref}
	p, err := New(api, "project", "us-central1")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.RotateSecret(t.Context(), ref, filament.Secret{Tenant: "tenant", Value: []byte("rotated")}); err != nil {
		t.Fatal(err)
	}
	if api.add.Parent != p.secretName(ref) || api.destroy.Name != p.secretName(ref)+"/versions/6" {
		t.Fatal("rotation changed identity or destroyed wrong version")
	}
	if strings.Join(api.calls, ",") != "read,add,destroy" {
		t.Fatal("old version destroyed before successful append")
	}
}

func TestFailedAppendDoesNotDestroyOldVersion(t *testing.T) {
	ref := filament.ConnectionSecretRef("tenant", "connection", "token", 1)
	api := &fakeAPI{ref: ref, addErr: errors.New("unavailable")}
	p, err := New(api, "project", "us-central1")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.RotateSecret(t.Context(), ref, filament.Secret{Tenant: "tenant", Value: []byte("rotated")}); err == nil {
		t.Fatal("append error hidden")
	}
	if api.destroy != nil {
		t.Fatal("destroyed old version after append failure")
	}
}

func TestRotationReportsCleanupFailure(t *testing.T) {
	ref := filament.ConnectionSecretRef("tenant", "connection", "token", 1)
	cleanupErr := errors.New("destroy unavailable")
	api := &fakeAPI{ref: ref, destroyErr: cleanupErr}
	p, err := New(api, "project", "us-central1")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.RotateSecret(t.Context(), ref, filament.Secret{Tenant: "tenant", Value: []byte("rotated")}); !errors.Is(err, cleanupErr) {
		t.Fatal("cleanup failure hidden")
	}
	if api.add == nil || api.destroy == nil {
		t.Fatal("rotation did not attempt cleanup after append")
	}
}
