package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	secretsqlite "github.com/galaxy-io/filament/secret/sqlite"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/worker"
)

func TestCreateConnectionPinsDefaultConnectorVersion(t *testing.T) {
	store := sqlite.NewMemory()
	sources := registry.NewSources()
	sources.Register("example@v1", func() filament.Source { return &catalogSource{name: "example@v1"} })
	sources.RegisterAlias("example", "example@v1")
	api := New(worker.Local(sources, registry.NewSinks()), store, nil, nil)
	created, err := api.CreateConnection(testCtx(), connect.NewRequest(&ingestionv1.CreateConnectionRequest{
		Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, Name: "example", Connector: "example",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := created.Msg.GetConnection().GetConnector(); got != "example@v1" {
		t.Fatalf("response connector=%q", got)
	}
	stored, err := store.LoadConnection(testCtx(), filament.DefaultTenantID, created.Msg.GetConnection().GetId())
	if err != nil {
		t.Fatal(err)
	}
	if stored.Connector != "example@v1" {
		t.Fatalf("stored connector=%q", stored.Connector)
	}

	// A subsequent binary may select v2 by default. The persisted connection
	// must still select v1 without relying on that default alias.
	upgraded := registry.NewSources()
	for _, version := range []string{"v1", "v2"} {
		upgraded.Register("example@"+version, func() filament.Source { return &catalogSource{name: "example@" + version} })
	}
	upgraded.RegisterAlias("example", "example@v2")
	source, err := upgraded.Resolve(stored.Connector)
	if err != nil || source.Spec().Name != "example@v1" {
		t.Fatalf("pinned resolution=%v err=%v", source, err)
	}
}

type refreshCredentialSource struct{ catalogSource }

func (s *refreshCredentialSource) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{Name: "refresh-fixture", Config: filament.ConfigSchema{Fields: []filament.ConfigField{{Name: "refresh_token", Type: filament.FieldSecret, Required: true}}}}
}

type unavailableSecrets struct {
	filament.Secrets
	blocked      string
	blockedReads int
}

func (s *unavailableSecrets) Read(ctx context.Context, ref string) (filament.Secret, error) {
	if ref == s.blocked {
		s.blockedReads++
		return filament.Secret{}, errors.New("secret temporarily unavailable")
	}
	return s.Secrets.Read(ctx, ref)
}

func TestConnectionCanReplaceUnavailableRefreshCredential(t *testing.T) {
	store := sqlite.NewMemory()
	defer store.Close()
	backend, err := secretsqlite.New(store.DB(), "test", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	secrets := &unavailableSecrets{Secrets: backend}
	sources := registry.NewSources()
	sources.Register("refresh-fixture", func() filament.Source { return &refreshCredentialSource{} })
	api := New(worker.Local(sources, registry.NewSinks()), store, nil, nil, WithSecrets(secrets))
	config, _ := structpb.NewStruct(map[string]any{"refresh_token": "initial"})
	created, err := api.CreateConnection(testCtx(), connect.NewRequest(&ingestionv1.CreateConnectionRequest{Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, Name: "refresh", Connector: "refresh-fixture", Config: config}))
	if err != nil {
		t.Fatal(err)
	}
	connection := created.Msg.GetConnection()
	secrets.blocked = connection.SecretRefs["refresh_token"]
	connection.Config, _ = structpb.NewStruct(map[string]any{"refresh_token": "fresh"})
	saved, err := api.UpdateConnection(testCtx(), connect.NewRequest(&ingestionv1.UpdateConnectionRequest{Connection: connection}))
	if err != nil {
		t.Fatal(err)
	}
	if secrets.blockedReads != 0 {
		t.Fatal("replacement attempted to read the interrupted credential")
	}
	ref := saved.Msg.Connection.SecretRefs["refresh_token"]
	if ref == secrets.blocked {
		t.Fatal("replacement reused interrupted reference")
	}
	value, err := secrets.Read(testCtx(), ref)
	if err != nil || string(value.Value) != "fresh" {
		t.Fatalf("replacement not persisted: %v", err)
	}
	if _, err := backend.Read(testCtx(), secrets.blocked); !errors.Is(err, filament.ErrNotFound) {
		t.Fatal("old credential not retired")
	}
}

func TestSecretSaveRetriesUseDistinctReferences(t *testing.T) {
	store := sqlite.NewMemory()
	defer store.Close()
	backend, err := secretsqlite.New(store.DB(), "test", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	api := New(nil, store, nil, nil, WithSecrets(backend))
	schema := (&refreshCredentialSource{}).Spec().Config
	var previous string
	for range 2 {
		refs := map[string]string{}
		cfg := map[string]any{"refresh_token": "fresh"}
		written, err := api.storeSecretFields(testCtx(), schema, string(filament.DefaultTenantID), "connection", 2, cfg, refs, true)
		if err != nil {
			t.Fatal(err)
		}
		ref := refs["refresh_token"]
		if ref == previous {
			t.Fatal("retry reused a failed attempt's reference")
		}
		previous = ref
		api.deleteSecretRefs(testCtx(), written)
	}
}

func TestPrepareKeyPairs(t *testing.T) {
	schema := []filament.ConfigField{{Name: "tunnel", Type: filament.FieldObject, Fields: []filament.ConfigField{
		{Name: "method", Type: filament.FieldEnum},
		{Name: "private_key", Type: filament.FieldSecret, PublicKeyField: "public_key", VisibleWhen: &filament.FieldCondition{Field: "method", Values: []string{"ssh"}}},
		{Name: "public_key", Type: filament.FieldString, ReadOnly: true},
	}}}
	tunnel := func(cfg map[string]any) map[string]any { return cfg["tunnel"].(map[string]any) }

	generated := map[string]any{"tunnel": map[string]any{"method": "ssh"}}
	if made, err := prepareKeyPairs(schema, generated, map[string]string{}, nil, ""); err != nil || !made {
		t.Fatalf("generated %v, %v", made, err)
	}
	private, _ := tunnel(generated)["private_key"].(string)
	public, _ := tunnel(generated)["public_key"].(string)
	if derived, err := sshPublicKey(private); err != nil || derived != public || !strings.HasPrefix(public, "ssh-ed25519 ") {
		t.Fatalf("generated pair: public %q, derived %q, %v", public, derived, err)
	}

	supplied := map[string]any{"tunnel": map[string]any{"method": "ssh", "private_key": private, "public_key": "ssh-ed25519 forged"}}
	if made, err := prepareKeyPairs(schema, supplied, map[string]string{}, nil, ""); err != nil || made {
		t.Fatalf("generated %v, %v", made, err)
	}
	if got := tunnel(supplied)["public_key"]; got != public {
		t.Fatalf("a supplied key must show its own public half: %v", got)
	}

	kept := map[string]any{"tunnel": map[string]any{"method": "ssh", "public_key": "ssh-ed25519 forged"}}
	stored := map[string]any{"tunnel": map[string]any{"public_key": public}}
	if made, err := prepareKeyPairs(schema, kept, map[string]string{"tunnel.private_key": "ref"}, stored, ""); err != nil || made {
		t.Fatalf("generated %v, %v", made, err)
	}
	if _, regenerated := tunnel(kept)["private_key"]; regenerated || tunnel(kept)["public_key"] != public {
		t.Fatalf("a stored key must keep its public half: %v", tunnel(kept))
	}

	direct := map[string]any{"tunnel": map[string]any{"method": "none"}}
	if _, err := prepareKeyPairs(schema, direct, map[string]string{}, nil, ""); err != nil || len(tunnel(direct)) != 1 {
		t.Fatalf("no key without a tunnel: %v, %v", tunnel(direct), err)
	}
	bad := map[string]any{"tunnel": map[string]any{"method": "ssh", "private_key": "not a key"}}
	if _, err := prepareKeyPairs(schema, bad, map[string]string{}, nil, ""); err == nil {
		t.Fatal("an unreadable private key was accepted")
	}
}

// keyPairSource has a tunnel section whose key the server can generate. It
// refuses a tunnel with no key, as the database connectors do, and its live
// test stands in for a bastion that has not been given the key.
type keyPairSource struct {
	catalogSource
	probes *atomic.Int32
}

func (s *keyPairSource) Spec() filament.ConnectorSpec {
	ssh := &filament.FieldCondition{Field: "method", Values: []string{"ssh"}}
	return filament.ConnectorSpec{Name: "keypair-fixture", Config: filament.ConfigSchema{Fields: []filament.ConfigField{
		{Name: "host", Type: filament.FieldString, Scope: filament.ScopeConnection},
		{Name: "tunnel", Type: filament.FieldObject, Scope: filament.ScopeConnection, Fields: []filament.ConfigField{
			{Name: "method", Type: filament.FieldEnum, Scope: filament.ScopeConnection, Enum: []filament.EnumOption{{Value: "none"}, {Value: "ssh"}}},
			{Name: "private_key", Type: filament.FieldSecret, PublicKeyField: "public_key", Scope: filament.ScopeConnection, VisibleWhen: ssh},
			{Name: "public_key", Type: filament.FieldString, ReadOnly: true, Scope: filament.ScopeConnection, VisibleWhen: ssh},
		}},
	}}}
}

func (s *keyPairSource) Validate(cfg filament.Config) error {
	if cfg.Sub("tunnel").Secret("private_key") == "" {
		return fmt.Errorf("%w: tunnel.private_key is required", filament.ErrConfigure)
	}
	return nil
}

func (s *keyPairSource) TestConnection(context.Context, filament.Config) error {
	s.probes.Add(1)
	return fmt.Errorf("%w: the bastion refused the key", filament.ErrConfigure)
}

func TestGeneratedKeyPairThroughTheConnectionLifecycle(t *testing.T) {
	store := sqlite.NewMemory()
	defer store.Close()
	secrets, err := secretsqlite.New(store.DB(), "test", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	probes := &atomic.Int32{}
	sources := registry.NewSources()
	sources.Register("keypair-fixture", func() filament.Source { return &keyPairSource{probes: probes} })
	api := New(worker.Local(sources, registry.NewSinks()), store, nil, nil, WithSecrets(secrets))
	const kind = ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE
	config := func(host string) *structpb.Struct {
		out, err := structpb.NewStruct(map[string]any{"host": host, "tunnel": map[string]any{"method": "ssh"}})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	publicKey := func(connection *ingestionv1.Connection) string {
		return connection.GetConfig().AsMap()["tunnel"].(map[string]any)["public_key"].(string)
	}
	validate := func(connectionID string, cfg *structpb.Struct) bool {
		response, err := api.ValidateConfig(testCtx(), connect.NewRequest(&ingestionv1.ValidateConfigRequest{Kind: kind, Connector: "keypair-fixture", Config: cfg, ConnectionId: connectionID}))
		if err != nil {
			t.Fatal(err)
		}
		return response.Msg.GetValid()
	}

	// A new connection with no key validates without a live test: the key
	// does not exist yet, so no bastion can hold it.
	if !validate("", config("db.internal")) || probes.Load() != 0 {
		t.Fatalf("blank key: probes = %d", probes.Load())
	}
	created, err := api.CreateConnection(testCtx(), connect.NewRequest(&ingestionv1.CreateConnectionRequest{Kind: kind, Name: "tunneled", Connector: "keypair-fixture", Config: config("db.internal")}))
	if err != nil {
		t.Fatal(err)
	}
	connection := created.Msg.GetConnection()
	first, firstRef := publicKey(connection), connection.GetSecretRefs()["tunnel.private_key"]
	if !strings.HasPrefix(first, "ssh-ed25519 ") || firstRef == "" {
		t.Fatalf("created with public key %q, ref %q", first, firstRef)
	}

	// Once the key is saved the live test runs against it.
	if validate(connection.GetId(), config("db.internal")) || probes.Load() != 1 {
		t.Fatalf("saved key: probes = %d", probes.Load())
	}

	// An edit that leaves the connection fields alone keeps the key, whether
	// or not the caller sends the read-only public half back.
	connection.Config = config("db.internal")
	unchanged, err := api.UpdateConnection(testCtx(), connect.NewRequest(&ingestionv1.UpdateConnectionRequest{Connection: connection}))
	if err != nil {
		t.Fatal(err)
	}
	connection = unchanged.Msg.GetConnection()
	if publicKey(connection) != first || connection.GetSecretRefs()["tunnel.private_key"] != firstRef {
		t.Fatalf("an unchanged edit replaced the key: %q", publicKey(connection))
	}

	// Changing a connection field drops the saved key, which the caller never
	// saw and cannot supply again, so the update gets a new pair.
	if !validate(connection.GetId(), config("other.internal")) || probes.Load() != 1 {
		t.Fatalf("changed host: probes = %d", probes.Load())
	}
	connection.Config = config("other.internal")
	changed, err := api.UpdateConnection(testCtx(), connect.NewRequest(&ingestionv1.UpdateConnectionRequest{Connection: connection}))
	if err != nil {
		t.Fatalf("editing a connection field: %v", err)
	}
	connection = changed.Msg.GetConnection()
	second, secondRef := publicKey(connection), connection.GetSecretRefs()["tunnel.private_key"]
	if !strings.HasPrefix(second, "ssh-ed25519 ") || second == first || secondRef == "" || secondRef == firstRef {
		t.Fatalf("after the edit: public key %q, ref %q", second, secondRef)
	}
	private, err := secrets.Read(testCtx(), secondRef)
	if err != nil {
		t.Fatal(err)
	}
	if derived, err := sshPublicKey(string(private.Value)); err != nil || derived != second {
		t.Fatalf("the stored key does not match the public half shown: %v", err)
	}
}
