package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite/sqlcgen"
)

// CreateConnection stores a new connection at version 1, rejecting a duplicate ID.
func (s *Store) CreateConnection(ctx context.Context, c filament.Connection) (filament.Connection, error) {
	if c.ID == "" {
		return filament.Connection{}, fmt.Errorf("datastore/sqlite: connection id is required")
	}
	configJSON, refsJSON, err := marshalConnectionConfig(c)
	if err != nil {
		return filament.Connection{}, err
	}
	if err := s.ensureTenantRow(ctx, c.Tenant); err != nil {
		return filament.Connection{}, fmt.Errorf("datastore/sqlite: ensure tenant: %w", err)
	}
	now := nowMillis()
	err = s.q.CreateConnection(ctx, sqlcgen.CreateConnectionParams{
		ConnectionID: c.ID, TenantID: c.Tenant, Kind: kindToDB(c.Kind), Name: c.Name, Connector: c.Connector,
		Config: configJSON, SecretRefs: refsJSON, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return filament.Connection{}, fmt.Errorf("datastore/sqlite: create connection: %w", err)
	}
	return s.LoadConnection(ctx, filament.TenantID(c.Tenant), c.ID)
}

// UpdateConnection replaces a stored connection, enforcing optimistic version matching.
func (s *Store) UpdateConnection(ctx context.Context, c filament.Connection) (filament.Connection, error) {
	if c.ID == "" {
		return filament.Connection{}, fmt.Errorf("datastore/sqlite: connection id is required")
	}
	configJSON, refsJSON, err := marshalConnectionConfig(c)
	if err != nil {
		return filament.Connection{}, err
	}
	newVersion, err := s.q.UpdateConnection(ctx, sqlcgen.UpdateConnectionParams{
		Name: c.Name, Connector: c.Connector, Config: configJSON, SecretRefs: refsJSON,
		TenantID: c.Tenant, ConnectionID: c.ID, ExpectedVersion: c.Version, UpdatedAt: nowMillis(),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return filament.Connection{}, fmt.Errorf("update connection %q at version %d: %w", c.ID, c.Version, filament.ErrVersionConflict)
	}
	if err != nil {
		return filament.Connection{}, fmt.Errorf("datastore/sqlite: update connection: %w", err)
	}
	c.Version = newVersion
	return s.LoadConnection(ctx, filament.TenantID(c.Tenant), c.ID)
}

// LoadConnection returns the connection with the given ID, including
// soft-deleted ones so callers can still read a deleted connection's
// metadata. DeletedAt tells them apart.
func (s *Store) LoadConnection(ctx context.Context, tenant filament.TenantID, id string) (filament.Connection, error) {
	row, err := s.q.GetConnection(ctx, sqlcgen.GetConnectionParams{TenantID: string(tenant), ConnectionID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return filament.Connection{}, fmt.Errorf("get connection %q: %w", id, filament.ErrNotFound)
	}
	if err != nil {
		return filament.Connection{}, fmt.Errorf("datastore/sqlite: get connection: %w", err)
	}
	return connectionFromRow(row.ID, row.TenantID, row.Kind, row.Name, row.Connector, row.Config, row.SecretRefs,
		row.Version, row.CreatedAt, row.UpdatedAt, row.DeletedAt, row.CreatedByUserID, row.UpdatedByUserID, row.DeletedByUserID)
}

// ListConnections returns one filtered, sorted page and its pre-page total.
func (s *Store) ListConnections(ctx context.Context, f filament.ConnectionFilter) ([]filament.Connection, int, error) {
	kind := ""
	if f.Kind != filament.ConnectorKindUnspecified {
		kind = kindToDB(f.Kind)
	}
	search := strings.TrimSpace(f.Search)
	count, err := s.q.CountConnections(ctx, sqlcgen.CountConnectionsParams{
		TenantID: f.Tenant, Kind: kind, KindFilter: kind, IncludeDeleted: f.IncludeDeleted,
		Search: search, NameSearch: search, ConnectorSearch: search,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("datastore/sqlite: count connections: %w", err)
	}
	// The sort and page clauses are assembled in Go: sqlc's sqlite engine
	// cannot bind parameters inside ORDER BY CASE expressions.
	// #nosec G202 -- concatenated fragments come from sortClause/pageClause's fixed vocabulary, never caller input
	query := `SELECT ` + connectionColumns + ` FROM connections
		WHERE tenant_id = ?1
		  AND (?2 = '' OR kind = ?2)
		  AND (?3 OR is_deleted = 0)
		  AND (?4 = '' OR instr(lower(name), lower(?4)) > 0 OR instr(lower(connector), lower(?4)) > 0)` +
		sortClause(f.SortBy, f.SortDescending) + pageClause(f.Limit, f.Offset)
	rows, err := s.db.QueryContext(ctx, query, f.Tenant, kind, f.IncludeDeleted, search)
	if err != nil {
		return nil, 0, fmt.Errorf("datastore/sqlite: list connections: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []filament.Connection
	for rows.Next() {
		var (
			id, tenant, rowKind, name, connector, config, refs, createdBy, updatedBy, deletedBy string
			version, createdAt, updatedAt                                                       int64
			deletedAt                                                                           sql.NullInt64
		)
		if err := rows.Scan(&id, &tenant, &rowKind, &name, &connector, &config, &refs, &version,
			&createdAt, &updatedAt, &deletedAt, &createdBy, &updatedBy, &deletedBy); err != nil {
			return nil, 0, fmt.Errorf("datastore/sqlite: scan connection: %w", err)
		}
		c, err := connectionFromRow(id, tenant, rowKind, name, connector, config, refs,
			version, createdAt, updatedAt, deletedAt, createdBy, updatedBy, deletedBy)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("datastore/sqlite: list connections: %w", err)
	}
	return out, int(count), nil
}

// DeleteConnection soft-deletes the connection with the given ID; deleting a
// missing ID is a no-op. The name is freed for reuse by the partial unique
// index.
func (s *Store) DeleteConnection(ctx context.Context, tenant filament.TenantID, id string) error {
	now := nowMillis()
	err := s.q.DeleteConnection(ctx, sqlcgen.DeleteConnectionParams{
		TenantID: string(tenant), ConnectionID: id, DeleteStamp: deleteStamp(now),
		DeletedAt: sql.NullInt64{Int64: now, Valid: true}, UpdatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("datastore/sqlite: delete connection: %w", err)
	}
	return nil
}

func connectionFromRow(id, tenant, kind, name, connector, configJSON, refsJSON string, version, createdAt, updatedAt int64,
	deletedAt sql.NullInt64, createdBy, updatedBy, deletedBy string,
) (filament.Connection, error) {
	cfg := map[string]any{}
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			return filament.Connection{}, fmt.Errorf("datastore/sqlite: unmarshal connection config: %w", err)
		}
	}
	refs := map[string]string{}
	if refsJSON != "" {
		if err := json.Unmarshal([]byte(refsJSON), &refs); err != nil {
			return filament.Connection{}, fmt.Errorf("datastore/sqlite: unmarshal connection secret_refs: %w", err)
		}
	}
	return filament.Connection{
		ID: id, Tenant: tenant, Kind: kindFromDB(kind), Name: name, Connector: connector,
		Config: cfg, SecretRefs: refs, Version: version,
		CreatedAt: createdAt, UpdatedAt: updatedAt, DeletedAt: deletedAt.Int64,
		CreatedByUserID: createdBy, UpdatedByUserID: updatedBy, DeletedByUserID: deletedBy,
	}, nil
}

func marshalConnectionConfig(c filament.Connection) (string, string, error) {
	cfg := c.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return "", "", fmt.Errorf("datastore/sqlite: marshal connection config: %w", err)
	}
	refs := c.SecretRefs
	if refs == nil {
		refs = map[string]string{}
	}
	refsJSON, err := json.Marshal(refs)
	if err != nil {
		return "", "", fmt.Errorf("datastore/sqlite: marshal connection secret_refs: %w", err)
	}
	return string(configJSON), string(refsJSON), nil
}

func kindToDB(kind filament.ConnectorKind) string {
	if kind == filament.ConnectorKindSink {
		return "sink"
	}
	return "source"
}

func kindFromDB(kind string) filament.ConnectorKind {
	switch kind {
	case "sink":
		return filament.ConnectorKindSink
	case "source":
		return filament.ConnectorKindSource
	default:
		return filament.ConnectorKindUnspecified
	}
}
