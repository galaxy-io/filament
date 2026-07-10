package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	ingestion "github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

// ConnectionStore is a Postgres-backed store for reusable, tenant-scoped
// Connections the sources and sinks a pipeline node references by id. 
type ConnectionStore struct {
	q *sqlcgen.Queries
}

func NewConnectionStore(pool *pgxpool.Pool) *ConnectionStore {
	return &ConnectionStore{q: sqlcgen.New(pool)}
}

// Create inserts a new connection at version 1, using the id supplied by the
// caller
func (s *ConnectionStore) Create(ctx context.Context, c *ingestionv1.Connection) (*ingestionv1.Connection, error) {
	if c.GetId() == "" {
		return nil, fmt.Errorf("datastore/postgres: connection id is required")
	}
	configJSON, refsJSON, err := marshalConnectionConfig(c)
	if err != nil {
		return nil, err
	}
	err = s.q.CreateConnection(ctx, sqlcgen.CreateConnectionParams{
		ConnectionID: c.GetId(),
		TenantID:     c.GetTenant(),
		Kind:         providerKindToDB(c.GetKind()),
		Name:         c.GetName(),
		Provider:     c.GetProvider(),
		Config:       configJSON,
		SecretRefs:   refsJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: create connection: %w", err)
	}
	out := cloneConnection(c)
	out.Version = 1
	return out, nil
}

// Update applies c if c.Version matches the stored version, then returns the new
// row (version+1). Returns ErrVersionConflict on mismatch or a missing id.
func (s *ConnectionStore) Update(ctx context.Context, c *ingestionv1.Connection) (*ingestionv1.Connection, error) {
	if c.GetId() == "" {
		return nil, fmt.Errorf("datastore/postgres: connection id is required")
	}
	configJSON, refsJSON, err := marshalConnectionConfig(c)
	if err != nil {
		return nil, err
	}
	newVersion, err := s.q.UpdateConnection(ctx, sqlcgen.UpdateConnectionParams{
		Name:            c.GetName(),
		Provider:        c.GetProvider(),
		Config:          configJSON,
		SecretRefs:      refsJSON,
		ConnectionID:    c.GetId(),
		ExpectedVersion: c.GetVersion(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("update connection %q at version %d: %w", c.GetId(), c.GetVersion(), ErrVersionConflict)
		}
		return nil, fmt.Errorf("datastore/postgres: update connection: %w", err)
	}
	out := cloneConnection(c)
	out.Version = newVersion
	return out, nil
}

func (s *ConnectionStore) Get(ctx context.Context, id string) (*ingestionv1.Connection, error) {
	row, err := s.q.GetConnection(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("get connection %q: %w", id, ingestion.ErrNotFound)
		}
		return nil, fmt.Errorf("datastore/postgres: get connection: %w", err)
	}
	return connectionFromRow(row.ConnectionID, row.TenantID, row.Kind, row.Name, row.Provider, row.Config, row.SecretRefs, row.Version)
}

func (s *ConnectionStore) List(ctx context.Context, tenant string, kind ingestionv1.ProviderKind) ([]*ingestionv1.Connection, error) {
	kindFilter := sqlcgen.NullConnectionKind{}
	if kind != ingestionv1.ProviderKind_PROVIDER_KIND_UNSPECIFIED {
		kindFilter = sqlcgen.NullConnectionKind{ConnectionKind: providerKindToDB(kind), Valid: true}
	}
	rows, err := s.q.ListConnections(ctx, sqlcgen.ListConnectionsParams{TenantID: tenant, Kind: kindFilter})
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: list connections: %w", err)
	}
	out := make([]*ingestionv1.Connection, len(rows))
	for i, row := range rows {
		conn, err := connectionFromRow(row.ConnectionID, row.TenantID, row.Kind, row.Name, row.Provider, row.Config, row.SecretRefs, row.Version)
		if err != nil {
			return nil, err
		}
		out[i] = conn
	}
	return out, nil
}

func (s *ConnectionStore) Delete(ctx context.Context, id string) error {
	if err := s.q.DeleteConnection(ctx, id); err != nil {
		return fmt.Errorf("datastore/postgres: delete connection: %w", err)
	}
	return nil
}

func marshalConnectionConfig(c *ingestionv1.Connection) (configJSON, refsJSON []byte, err error) {
	cfg := c.GetConfig()
	if cfg == nil {
		cfg = &structpb.Struct{}
	}
	configJSON, err = protojson.Marshal(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("datastore/postgres: marshal connection config: %w", err)
	}
	refs := c.GetSecretRefs()
	if refs == nil {
		refs = map[string]string{}
	}
	refsJSON, err = json.Marshal(refs)
	if err != nil {
		return nil, nil, fmt.Errorf("datastore/postgres: marshal connection secret_refs: %w", err)
	}
	return configJSON, refsJSON, nil
}

func providerKindToDB(kind ingestionv1.ProviderKind) sqlcgen.ConnectionKind {
	switch kind {
	case ingestionv1.ProviderKind_PROVIDER_KIND_SINK:
		return sqlcgen.ConnectionKindSink
	default:
		return sqlcgen.ConnectionKindSource
	}
}

func providerKindFromDB(kind sqlcgen.ConnectionKind) ingestionv1.ProviderKind {
	switch kind {
	case sqlcgen.ConnectionKindSink:
		return ingestionv1.ProviderKind_PROVIDER_KIND_SINK
	case sqlcgen.ConnectionKindSource:
		return ingestionv1.ProviderKind_PROVIDER_KIND_SOURCE
	default:
		return ingestionv1.ProviderKind_PROVIDER_KIND_UNSPECIFIED
	}
}

func connectionFromRow(id, tenant string, kind sqlcgen.ConnectionKind, name, provider string, configJSON, refsJSON []byte, version int64) (*ingestionv1.Connection, error) {
	cfg := &structpb.Struct{}
	if len(configJSON) > 0 {
		if err := protojson.Unmarshal(configJSON, cfg); err != nil {
			return nil, fmt.Errorf("datastore/postgres: unmarshal connection config: %w", err)
		}
	}
	refs := map[string]string{}
	if len(refsJSON) > 0 {
		if err := json.Unmarshal(refsJSON, &refs); err != nil {
			return nil, fmt.Errorf("datastore/postgres: unmarshal connection secret_refs: %w", err)
		}
	}
	return &ingestionv1.Connection{
		Id:         id,
		Tenant:     tenant,
		Kind:       providerKindFromDB(kind),
		Name:       name,
		Provider:   provider,
		Config:     cfg,
		SecretRefs: refs,
		Version:    version,
	}, nil
}

func cloneConnection(c *ingestionv1.Connection) *ingestionv1.Connection {
	return proto.Clone(c).(*ingestionv1.Connection)
}
