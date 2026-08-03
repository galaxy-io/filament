package server_test

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/server"
)

type fakeSecrets struct {
	values map[string]filament.Secret
}

func newFakeSecrets() *fakeSecrets {
	return &fakeSecrets{values: map[string]filament.Secret{}}
}

func (f *fakeSecrets) Read(_ context.Context, ref string) (filament.Secret, error) {
	secret, ok := f.values[ref]
	if !ok {
		return filament.Secret{}, fmt.Errorf("secret %q not found", ref)
	}
	return secret, nil
}

func (f *fakeSecrets) Write(_ context.Context, ref string, s filament.Secret) error {
	f.values[ref] = s
	return nil
}

func (f *fakeSecrets) Delete(_ context.Context, ref string) error {
	delete(f.values, ref)
	return nil
}

func (f *fakeSecrets) Name() string { return "fake" }

type stubSource struct {
	testedConfigs []map[string]any
}

func (s *stubSource) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name: "stubsrc",
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "host", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection},
			{Name: "password", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection},
		}},
	}
}

func (s *stubSource) Validate(cfg filament.Config) error {
	if cfg.String("host") == "" {
		return fmt.Errorf("host is required")
	}
	if cfg.Secret("password") == "" {
		return fmt.Errorf("password is required")
	}
	return nil
}

func (s *stubSource) Configure(context.Context, filament.Config) error { return nil }

func (s *stubSource) Extract(context.Context, filament.RecordSink, filament.ExtractOpts) error {
	return nil
}

func (s *stubSource) Teardown(context.Context) error { return nil }

func (s *stubSource) TestConnection(_ context.Context, cfg filament.Config) error {
	s.testedConfigs = append(s.testedConfigs, cfg.Raw())
	return nil
}

func newConnectionTestServer(t *testing.T) (*server.Server, *fakeSecrets, *stubSource) {
	t.Helper()
	stub := &stubSource{}
	sources := registry.NewSources()
	sources.Register("stubsrc", func() filament.Source { return stub })
	secrets := newFakeSecrets()
	return server.New(sources, registry.NewSinks(), memory.New(), nil, nil, server.WithSecrets(secrets)), secrets, stub
}

func mustStruct(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatalf("structpb.NewStruct: %v", err)
	}
	return s
}

func createStubConnection(t *testing.T, srv *server.Server) *ingestionv1.Connection {
	t.Helper()
	resp, err := srv.CreateConnection(context.Background(), connect.NewRequest(&ingestionv1.CreateConnectionRequest{
		Kind:      ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
		Name:      "conn",
		Connector: "stubsrc",
		Config:    mustStruct(t, map[string]any{"host": "h1", "password": "pw1"}),
	}))
	if err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}
	return resp.Msg.GetConnection()
}

func updateRequest(t *testing.T, conn *ingestionv1.Connection, config map[string]any, refs map[string]string) *connect.Request[ingestionv1.UpdateConnectionRequest] {
	t.Helper()
	return connect.NewRequest(&ingestionv1.UpdateConnectionRequest{Connection: &ingestionv1.Connection{
		Id:         conn.GetId(),
		TenantId:   conn.GetTenantId(),
		Kind:       conn.GetKind(),
		Name:       conn.GetName(),
		Connector:  conn.GetConnector(),
		Config:     mustStruct(t, config),
		SecretRefs: refs,
		Version:    conn.GetVersion(),
	}})
}

func TestUpdateConnectionPreservesUntouchedSecret(t *testing.T) {
	srv, secrets, _ := newConnectionTestServer(t)
	conn := createStubConnection(t, srv)
	ref := conn.GetSecretRefs()["password"]
	if ref == "" {
		t.Fatal("expected a password secret ref after create")
	}

	resp, err := srv.UpdateConnection(context.Background(), updateRequest(t, conn, map[string]any{"host": "h2"}, conn.GetSecretRefs()))
	if err != nil {
		t.Fatalf("UpdateConnection: %v", err)
	}
	next := resp.Msg.GetConnection()
	if next.GetVersion() != 2 {
		t.Fatalf("version = %d, want 2", next.GetVersion())
	}
	if next.GetSecretRefs()["password"] != ref {
		t.Fatalf("password ref = %q, want unchanged %q", next.GetSecretRefs()["password"], ref)
	}
	secret, err := secrets.Read(context.Background(), ref)
	if err != nil {
		t.Fatalf("secret deleted: %v", err)
	}
	if string(secret.Value) != "pw1" {
		t.Fatalf("secret = %q, want pw1", secret.Value)
	}
}

func TestUpdateConnectionIgnoresClearedSecret(t *testing.T) {
	srv, secrets, _ := newConnectionTestServer(t)
	conn := createStubConnection(t, srv)
	ref := conn.GetSecretRefs()["password"]

	resp, err := srv.UpdateConnection(context.Background(), updateRequest(t, conn, map[string]any{"host": "h2", "password": ""}, conn.GetSecretRefs()))
	if err != nil {
		t.Fatalf("UpdateConnection: %v", err)
	}
	next := resp.Msg.GetConnection()
	if next.GetSecretRefs()["password"] != ref {
		t.Fatalf("password ref = %q, want unchanged %q", next.GetSecretRefs()["password"], ref)
	}
	secret, err := secrets.Read(context.Background(), ref)
	if err != nil {
		t.Fatalf("secret deleted: %v", err)
	}
	if string(secret.Value) != "pw1" {
		t.Fatalf("secret = %q, want pw1", secret.Value)
	}
	if _, present := next.GetConfig().AsMap()["password"]; present {
		t.Fatal("blank secret persisted in config")
	}
}

func TestUpdateConnectionReplacesSecret(t *testing.T) {
	srv, secrets, _ := newConnectionTestServer(t)
	conn := createStubConnection(t, srv)
	oldRef := conn.GetSecretRefs()["password"]

	resp, err := srv.UpdateConnection(context.Background(), updateRequest(t, conn, map[string]any{"host": "h1", "password": "pw2"}, conn.GetSecretRefs()))
	if err != nil {
		t.Fatalf("UpdateConnection: %v", err)
	}
	next := resp.Msg.GetConnection()
	newRef := next.GetSecretRefs()["password"]
	if newRef == oldRef {
		t.Fatal("expected a new versioned secret ref")
	}
	if _, err := secrets.Read(context.Background(), oldRef); err == nil {
		t.Fatal("expected replaced secret ref to be deleted")
	}
	secret, err := secrets.Read(context.Background(), newRef)
	if err != nil {
		t.Fatalf("read new secret: %v", err)
	}
	if string(secret.Value) != "pw2" {
		t.Fatalf("secret = %q, want pw2", secret.Value)
	}
	if _, present := next.GetConfig().AsMap()["password"]; present {
		t.Fatal("plaintext secret persisted in config")
	}
}

func TestUpdateConnectionWithoutSecretRefsDeletesSecrets(t *testing.T) {
	srv, secrets, _ := newConnectionTestServer(t)
	conn := createStubConnection(t, srv)
	ref := conn.GetSecretRefs()["password"]

	if _, err := srv.UpdateConnection(context.Background(), updateRequest(t, conn, map[string]any{"host": "h1"}, nil)); err != nil {
		t.Fatalf("UpdateConnection: %v", err)
	}
	if _, err := secrets.Read(context.Background(), ref); err == nil {
		t.Fatal("expected orphaned secret ref to be deleted")
	}
}

func TestUpdateConnectionVersionConflict(t *testing.T) {
	srv, _, _ := newConnectionTestServer(t)
	conn := createStubConnection(t, srv)

	stale := updateRequest(t, conn, map[string]any{"host": "h2"}, conn.GetSecretRefs())
	stale.Msg.Connection.Version = conn.GetVersion() + 5
	_, err := srv.UpdateConnection(context.Background(), stale)
	if connect.CodeOf(err) != connect.CodeAborted {
		t.Fatalf("code = %v, want aborted", connect.CodeOf(err))
	}
}

func TestUpdateConnectionRejectsConnectorChange(t *testing.T) {
	srv, _, _ := newConnectionTestServer(t)
	conn := createStubConnection(t, srv)

	changed := updateRequest(t, conn, map[string]any{"host": "h1"}, conn.GetSecretRefs())
	changed.Msg.Connection.Connector = "othersrc"
	_, err := srv.UpdateConnection(context.Background(), changed)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("code = %v, want invalid_argument", connect.CodeOf(err))
	}

	kinded := updateRequest(t, conn, map[string]any{"host": "h1"}, conn.GetSecretRefs())
	kinded.Msg.Connection.Kind = ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK
	_, err = srv.UpdateConnection(context.Background(), kinded)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("code = %v, want invalid_argument", connect.CodeOf(err))
	}
}

func TestValidateConfigFillsStoredSecrets(t *testing.T) {
	srv, _, stub := newConnectionTestServer(t)
	conn := createStubConnection(t, srv)

	resp, err := srv.ValidateConfig(context.Background(), connect.NewRequest(&ingestionv1.ValidateConfigRequest{
		Kind:         ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
		Connector:    "stubsrc",
		Config:       mustStruct(t, map[string]any{"host": "h2", "password": ""}),
		Live:         true,
		ConnectionId: conn.GetId(),
	}))
	if err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
	if !resp.Msg.GetValid() {
		t.Fatalf("valid = false, errors = %v", resp.Msg.GetErrors())
	}
	if len(stub.testedConfigs) != 1 {
		t.Fatalf("TestConnection calls = %d, want 1", len(stub.testedConfigs))
	}
	if got := stub.testedConfigs[0]["password"]; got != "pw1" {
		t.Fatalf("probe password = %v, want stored pw1", got)
	}
}

func TestValidateConfigPrefersTypedSecret(t *testing.T) {
	srv, _, stub := newConnectionTestServer(t)
	conn := createStubConnection(t, srv)

	resp, err := srv.ValidateConfig(context.Background(), connect.NewRequest(&ingestionv1.ValidateConfigRequest{
		Kind:         ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
		Connector:    "stubsrc",
		Config:       mustStruct(t, map[string]any{"host": "h2", "password": "typed"}),
		Live:         true,
		ConnectionId: conn.GetId(),
	}))
	if err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
	if !resp.Msg.GetValid() {
		t.Fatalf("valid = false, errors = %v", resp.Msg.GetErrors())
	}
	if got := stub.testedConfigs[0]["password"]; got != "typed" {
		t.Fatalf("probe password = %v, want typed", got)
	}
}
