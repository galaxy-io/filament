package server

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/registry"
	secretsqlite "github.com/galaxy-io/filament/secret/sqlite"
	"github.com/galaxy-io/filament/worker"
)

const (
	savedConnector = "credential-source"
	otherConnector = "other-source"
	savedHost      = "db-a.example"
	savedPassword  = "saved-password"
	otherHost      = "db-b.example"
)

// receivedConfig is what one connector call was handed.
type receivedConfig struct{ connector, host, password string }

// recordingSource has a host and a secret password. It records the config of
// every live test (ValidateConfig) and every configure (DiscoverResources,
// GetResourceColumns), so a test can see where a saved credential went.
type recordingSource struct {
	name string
	seen *[]receivedConfig
}

func (s *recordingSource) record(cfg filament.Config) {
	*s.seen = append(*s.seen, receivedConfig{connector: s.name, host: cfg.String("host"), password: cfg.String("password")})
}

func (s *recordingSource) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{Name: s.name, Config: filament.ConfigSchema{Fields: []filament.ConfigField{
		{Name: "host", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection},
		{Name: "password", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection},
	}}}
}

func (s *recordingSource) Validate(filament.Config) error { return nil }

func (s *recordingSource) Configure(_ context.Context, cfg filament.Config) error {
	s.record(cfg)
	return nil
}

func (s *recordingSource) TestConnection(_ context.Context, cfg filament.Config) error {
	s.record(cfg)
	return nil
}

func (s *recordingSource) Extract(context.Context, filament.RecordSink, filament.ExtractOpts) error {
	return nil
}

func (s *recordingSource) Teardown(context.Context) error { return nil }

func (s *recordingSource) Discover(context.Context, filament.DiscoverOpts) (filament.DiscoverResult, error) {
	return filament.DiscoverResult{Resources: []filament.Resource{{Name: "users", Selectable: true}}}, nil
}

func (s *recordingSource) CursorColumns(context.Context, string) ([]filament.CursorColumn, error) {
	return []filament.CursorColumn{{SchemaField: filament.SchemaField{Name: "updated_at"}, Eligible: true}}, nil
}

// savedConnectionFixture is a server holding one saved connection whose
// password lives in the encrypted secret store.
type savedConnectionFixture struct {
	api          *Server
	store        *sqlite.Store
	secrets      *secretsqlite.Provider
	connectionID string
	passwordRef  string // the saved connection's password ref
	seen         *[]receivedConfig
}

// create saves another connection on the same server.
func (f savedConnectionFixture) create(t *testing.T, name string, cfg map[string]any) *ingestionv1.Connection {
	t.Helper()
	created, err := f.api.CreateConnection(testCtx(), connect.NewRequest(&ingestionv1.CreateConnectionRequest{
		Kind:      ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
		Name:      name,
		Connector: savedConnector,
		Config:    configStruct(t, cfg),
	}))
	if err != nil {
		t.Fatal(err)
	}
	return created.Msg.GetConnection()
}

func newSavedConnectionFixture(t *testing.T) savedConnectionFixture {
	t.Helper()
	store := sqlite.NewMemory()
	t.Cleanup(func() { _ = store.Close() })
	secrets, err := secretsqlite.New(store.DB(), "test", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	seen := &[]receivedConfig{}
	sources := registry.NewSources()
	for _, name := range []string{savedConnector, otherConnector} {
		sources.Register(name, func() filament.Source { return &recordingSource{name: name, seen: seen} })
	}
	// A plain sink so pipeline validation has somewhere to write.
	sinks := registry.NewSinks()
	sinks.Register("live-sink", func() filament.Sink { return &liveProbeSink{probes: &atomic.Int32{}} })
	api := New(worker.Local(sources, sinks), store, nil, nil, WithSecrets(secrets))
	f := savedConnectionFixture{api: api, store: store, secrets: secrets, seen: seen}

	connection := f.create(t, "primary", map[string]any{"host": savedHost, "password": savedPassword})
	// Guard against a vacuous pass: the password must live in the secret
	// store, so the handlers can only obtain it by resolving the ref.
	f.passwordRef = connection.GetSecretRefs()["password"]
	if f.passwordRef == "" {
		t.Fatalf("password was not stored as a secret ref: refs = %v", connection.GetSecretRefs())
	}
	f.connectionID = connection.GetId()
	*seen = nil // forget anything CreateConnection itself probed
	return f
}

func configStruct(t *testing.T, cfg map[string]any) *structpb.Struct {
	t.Helper()
	if cfg == nil {
		return nil
	}
	s, err := structpb.NewStruct(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// savedConnectionCall calls one RPC against the saved connection.
type savedConnectionCall func(t *testing.T, api *Server, connectionID string) error

func validateWith(connector string, cfg map[string]any) savedConnectionCall {
	return func(t *testing.T, api *Server, connectionID string) error {
		_, err := api.ValidateConfig(testCtx(), connect.NewRequest(&ingestionv1.ValidateConfigRequest{
			Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, Connector: connector,
			ConnectionId: connectionID, Config: configStruct(t, cfg),
		}))
		return err
	}
}

func discoverWith(connector string, cfg map[string]any) savedConnectionCall {
	return func(t *testing.T, api *Server, connectionID string) error {
		_, err := api.DiscoverResources(testCtx(), connect.NewRequest(&ingestionv1.DiscoverResourcesRequest{
			Connector: connector, ConnectionId: connectionID, Config: configStruct(t, cfg),
		}))
		return err
	}
}

func columnsWith(connector string, cfg map[string]any) savedConnectionCall {
	return func(t *testing.T, api *Server, connectionID string) error {
		_, err := api.GetResourceColumns(testCtx(), connect.NewRequest(&ingestionv1.GetResourceColumnsRequest{
			Connector: connector, ConnectionId: connectionID, Config: configStruct(t, cfg),
			Resources: []string{"users"},
		}))
		return err
	}
}

// validatePipelineWith validates a draft pipeline reading the saved connection
// with nodeConfig on its source node.
func validatePipelineWith(nodeConfig map[string]any) savedConnectionCall {
	return func(t *testing.T, api *Server, connectionID string) error {
		sink, err := api.CreateConnection(testCtx(), connect.NewRequest(&ingestionv1.CreateConnectionRequest{
			Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK, Name: "warehouse", Connector: "live-sink",
		}))
		if err != nil {
			t.Fatal(err)
		}
		_, err = api.ValidatePipeline(testCtx(), connect.NewRequest(&ingestionv1.ValidatePipelineRequest{Graph: &ingestionv1.PipelineGraph{
			Nodes: []*ingestionv1.PipelineNode{
				{Id: "src", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, ConnectionId: connectionID, Config: configStruct(t, nodeConfig)},
				{Id: "snk", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK, ConnectionId: sink.Msg.GetConnection().GetId()},
			},
			Edges: []*ingestionv1.PipelineEdge{{FromNode: "src", ToNode: "snk", Resource: "users"}},
		}}))
		return err
	}
}

func TestSavedConnectionConfig(t *testing.T) {
	newHost := map[string]any{"host": otherHost}
	// What the edit form sends when nothing changed: every field, with the
	// secret left blank because the UI never sees it.
	unchanged := map[string]any{"host": savedHost, "password": ""}

	cases := []struct {
		name string
		call savedConnectionCall
		// mustWork rows are normal use: the call must still succeed and the
		// connector must receive the saved credential.
		mustWork bool
	}{
		{name: "validate/new host", call: validateWith(savedConnector, newHost)},
		{name: "validate/different connector", call: validateWith(otherConnector, nil)},
		{name: "discover/new host", call: discoverWith("", newHost)},
		{name: "discover/different connector", call: discoverWith(otherConnector, nil)},
		{name: "columns/new host", call: columnsWith("", newHost)},
		{name: "columns/different connector", call: columnsWith(otherConnector, nil)},
		{name: "validate pipeline/new host in node config", call: validatePipelineWith(newHost)},

		{name: "validate/unchanged config", call: validateWith(savedConnector, unchanged), mustWork: true},
		{name: "discover/connection id only", call: discoverWith("", nil), mustWork: true},
		{name: "columns/connection id only", call: columnsWith("", nil), mustWork: true},
		{name: "validate pipeline/no node config", call: validatePipelineWith(nil), mustWork: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSavedConnectionFixture(t)
			// Rejecting the request is fine; what matters is the config each
			// connector received.
			err := tc.call(t, f.api, f.connectionID)

			for _, got := range *f.seen {
				if got.password == savedPassword && (got.host != savedHost || got.connector != savedConnector) {
					t.Errorf("saved password used with connector %q at host %q", got.connector, got.host)
				}
			}
			if tc.mustWork {
				if err != nil {
					t.Fatalf("normal use failed: %v", err)
				}
				want := receivedConfig{connector: savedConnector, host: savedHost, password: savedPassword}
				if !slices.Contains(*f.seen, want) {
					t.Fatalf("connector never received the saved credential; got %+v", *f.seen)
				}
			}
		})
	}
}

func TestUpdateConnectionSecretRefs(t *testing.T) {
	// seed stores value at ref for tenant and returns the ref.
	seed := func(tenant filament.TenantID, ref, value string) func(t *testing.T, f savedConnectionFixture) string {
		return func(t *testing.T, f savedConnectionFixture) string {
			if err := f.secrets.Write(testCtx(), ref, filament.Secret{Tenant: tenant, Value: []byte(value)}); err != nil {
				t.Fatal(err)
			}
			return ref
		}
	}
	const (
		secondary = "secondary" // a second connection, saved with otherHost
		primary   = "primary"   // the fixture's saved connection
	)
	// A blank password tells the server to keep using the saved secret.
	keepPassword := map[string]any{"host": otherHost}

	cases := []struct {
		name   string
		target string
		// submittedRef returns the ref submitted in place of the target's own
		// password ref; nil keeps the target's own ref. Unless the row is
		// mustWork, the target must end up still holding its own ref.
		submittedRef func(t *testing.T, f savedConnectionFixture) string
		config       map[string]any
		// mustWork rows are normal edits: the update must succeed and the
		// connector must then receive want.
		mustWork bool
		want     receivedConfig
	}{
		{
			name:         "secondary: another connection's ref",
			target:       secondary,
			submittedRef: func(_ *testing.T, f savedConnectionFixture) string { return f.passwordRef },
			config:       keepPassword,
		},
		{
			// Already rejected today; this row keeps it that way.
			name:         "secondary: another tenant's ref",
			target:       secondary,
			submittedRef: seed("other-tenant", "filament/other-tenant/conn-1/password/v1/x", savedPassword),
			config:       keepPassword,
		},
		{
			// The ref is primary's own; only the host changes.
			name:   "primary: new host, blank password",
			target: primary,
			config: keepPassword,
		},
		{
			name:     "secondary: own ref, unchanged host",
			target:   secondary,
			config:   keepPassword,
			mustWork: true,
			want:     receivedConfig{connector: savedConnector, host: otherHost, password: "secondary-password"},
		},
		{
			// Environment-style refs are the CLI's env:NAME feature and stay
			// allowed for now; restricting which variables the env provider
			// may read is a follow-up. The caller names one on their own
			// connection and it is used.
			name:         "secondary: env-style ref",
			target:       secondary,
			submittedRef: seed(filament.DefaultTenantID, "PG_PASSWORD", "env-password"),
			config:       keepPassword,
			mustWork:     true,
			want:         receivedConfig{connector: savedConnector, host: otherHost, password: "env-password"},
		},
		{
			name:     "primary: new host with new password",
			target:   primary,
			config:   map[string]any{"host": "new-db.internal", "password": "new-password"},
			mustWork: true,
			want:     receivedConfig{connector: savedConnector, host: "new-db.internal", password: "new-password"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newSavedConnectionFixture(t)
			var target *ingestionv1.Connection
			switch tc.target {
			case secondary:
				target = f.create(t, secondary, map[string]any{"host": otherHost, "password": "secondary-password"})
			case primary:
				got, err := f.api.GetConnection(testCtx(), connect.NewRequest(&ingestionv1.GetConnectionRequest{Id: f.connectionID}))
				if err != nil {
					t.Fatal(err)
				}
				target = got.Msg.GetConnection()
			}
			ownRef := target.GetSecretRefs()["password"]

			if tc.submittedRef != nil {
				target.SecretRefs["password"] = tc.submittedRef(t, f)
			}
			target.Config = configStruct(t, tc.config)
			_, err := f.api.UpdateConnection(testCtx(), connect.NewRequest(&ingestionv1.UpdateConnectionRequest{Connection: target}))
			if tc.mustWork && err != nil {
				t.Fatalf("normal update failed: %v", err)
			}
			// Rejecting the update is fine; the target must still use
			// only its own secret.
			if tc.submittedRef != nil && !tc.mustWork {
				stored, loadErr := f.store.LoadConnection(testCtx(), filament.DefaultTenantID, target.GetId())
				if loadErr != nil {
					t.Fatal(loadErr)
				}
				if got := stored.SecretRefs["password"]; got != ownRef {
					t.Errorf("password ref = %q, want the connection's own %q", got, ownRef)
				}
			}

			// End to end: whatever was saved, using the connection must only
			// ever use primary's password with primary's host.
			*f.seen = nil
			_ = discoverWith("", nil)(t, f.api, target.GetId())
			for _, got := range *f.seen {
				if got.password == savedPassword && got.host != savedHost {
					t.Errorf("saved password used with host %q by connection %q", got.host, target.GetName())
				}
			}
			if tc.mustWork && !slices.Contains(*f.seen, tc.want) {
				t.Errorf("connector never received %+v; got %+v", tc.want, *f.seen)
			}
		})
	}
}

func TestChangedConnectionField(t *testing.T) {
	schema := filament.ConfigSchema{Fields: []filament.ConfigField{
		{Name: "connection_method", Type: filament.FieldEnum, Default: "fields", Scope: filament.ScopeConnection},
		{Name: "dsn", Type: filament.FieldSecret, Scope: filament.ScopeConnection, VisibleWhen: &filament.FieldCondition{Field: "connection_method", Values: []string{"url"}}},
		{Name: "host", Type: filament.FieldString, Scope: filament.ScopeConnection, VisibleWhen: &filament.FieldCondition{Field: "connection_method", Values: []string{"fields"}}},
		{Name: "port", Type: filament.FieldInt, Default: 5432, Scope: filament.ScopeConnection, VisibleWhen: &filament.FieldCondition{Field: "connection_method", Values: []string{"fields"}}},
		{Name: "password", Type: filament.FieldSecret, Scope: filament.ScopeConnection, VisibleWhen: &filament.FieldCondition{Field: "connection_method", Values: []string{"fields"}}},
		{Name: "base_url", Type: filament.FieldString}, // no scope: counts as a connection field
		{Name: "table", Type: filament.FieldString, Scope: filament.ScopePipeline},
	}}
	saved := map[string]any{"connection_method": "fields", "host": savedHost, "base_url": "https://api.example"}

	cases := []struct {
		name      string
		requested map[string]any
		want      string // the changed field, or "" for none
		// anyField accepts any changed field: switching branch changes
		// several at once and which is reported first is unimportant.
		anyField bool
	}{
		{name: "nothing supplied", requested: nil},
		{name: "same values", requested: map[string]any{"host": savedHost, "base_url": "https://api.example"}},
		{name: "blank means not supplied", requested: map[string]any{"host": "", "base_url": ""}},
		{name: "default supplied explicitly", requested: map[string]any{"port": float64(5432)}},
		{name: "secret supplied", requested: map[string]any{"password": "new-password"}},
		{name: "pipeline field changed", requested: map[string]any{"table": "users"}},

		{name: "host changed", requested: map[string]any{"host": otherHost}, want: "host"},
		{name: "port changed from default", requested: map[string]any{"port": float64(6543)}, want: "port"},
		{name: "unscoped field changed", requested: map[string]any{"base_url": "https://api-b.example"}, want: "base_url"},
		{name: "unknown field added", requested: map[string]any{"proxy": otherHost}, want: "proxy"},
		// Switching to URL mode with a DSN: the DSN is the secret, so the
		// method itself is what marks the new destination.
		{name: "switch to url mode", requested: map[string]any{"connection_method": "url", "dsn": "postgres://db-b.example/db"}, anyField: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := changedConnectionField(schema, saved, tc.requested)
			switch {
			case tc.anyField && got == "":
				t.Fatal("changedConnectionField reported no change")
			case !tc.anyField && got != tc.want:
				t.Fatalf("changedConnectionField = %q, want %q", got, tc.want)
			}
		})
	}
}

// A new connection holds no secret refs yet.
func TestCreateConnectionSecretRefs(t *testing.T) {
	f := newSavedConnectionFixture(t)
	created, err := f.api.CreateConnection(testCtx(), connect.NewRequest(&ingestionv1.CreateConnectionRequest{
		Kind:       ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
		Name:       "new",
		Connector:  savedConnector,
		Config:     configStruct(t, map[string]any{"host": otherHost}),
		SecretRefs: map[string]string{"password": f.passwordRef},
	}))
	if err == nil {
		*f.seen = nil
		_ = discoverWith("", nil)(t, f.api, created.Msg.GetConnection().GetId())
		for _, got := range *f.seen {
			if got.password == savedPassword {
				t.Fatalf("new connection used primary's password at host %q", got.host)
			}
		}
		t.Fatalf("create accepted another connection's secret ref: refs = %v", created.Msg.GetConnection().GetSecretRefs())
	}
}
