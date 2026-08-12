package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

// CreateConnection stores a new connection at version 1, rejecting a duplicate ID.
func (s *Store) CreateConnection(ctx context.Context, c filament.Connection) (filament.Connection, error) {
	if c.ID == "" {
		return filament.Connection{}, fmt.Errorf("datastore/postgres: connection id is required")
	}
	configJSON, refsJSON, err := marshalConnectionConfig(c)
	if err != nil {
		return filament.Connection{}, err
	}
	err = s.q.CreateConnection(ctx, sqlcgen.CreateConnectionParams{ConnectionID: c.ID, TenantID: c.Tenant, Kind: connectionKindToDB(c.Kind), Name: c.Name, Provider: c.Connector, Config: configJSON, SecretRefs: refsJSON})
	if err != nil {
		return filament.Connection{}, fmt.Errorf("datastore/postgres: create connection: %w", err)
	}
	c.Version = 1
	return c, nil
}

// UpdateConnection replaces a stored connection, enforcing optimistic version matching.
func (s *Store) UpdateConnection(ctx context.Context, c filament.Connection) (filament.Connection, error) {
	if c.ID == "" {
		return filament.Connection{}, fmt.Errorf("datastore/postgres: connection id is required")
	}
	configJSON, refsJSON, err := marshalConnectionConfig(c)
	if err != nil {
		return filament.Connection{}, err
	}
	newVersion, err := s.q.UpdateConnection(ctx, sqlcgen.UpdateConnectionParams{Name: c.Name, Provider: c.Connector, Config: configJSON, SecretRefs: refsJSON, ConnectionID: c.ID, ExpectedVersion: c.Version})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return filament.Connection{}, fmt.Errorf("update connection %q at version %d: %w", c.ID, c.Version, filament.ErrVersionConflict)
		}
		return filament.Connection{}, fmt.Errorf("datastore/postgres: update connection: %w", err)
	}
	c.Version = newVersion
	return c, nil
}

// LoadConnection returns the connection with the given ID, including
// soft-deleted ones so callers can still read a deleted connection's metadata.
// DeletedAt tells them apart.
func (s *Store) LoadConnection(ctx context.Context, id string) (filament.Connection, error) {
	row, err := s.q.GetConnection(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return filament.Connection{}, fmt.Errorf("get connection %q: %w", id, filament.ErrNotFound)
		}
		return filament.Connection{}, fmt.Errorf("datastore/postgres: get connection: %w", err)
	}
	return connectionFromRow(row.ConnectionID, row.TenantID, row.Kind, row.Name, row.Provider, row.Config, row.SecretRefs, row.Version, row.DeletedAt)
}

// ListConnections returns connections matching the filter, sorted by ID.
func (s *Store) ListConnections(ctx context.Context, f filament.ConnectionFilter) ([]filament.Connection, error) {
	kind := sqlcgen.NullConnectionKind{}
	if f.Kind != filament.ConnectorKindUnspecified {
		kind = sqlcgen.NullConnectionKind{ConnectionKind: connectionKindToDB(f.Kind), Valid: true}
	}
	rows, err := s.q.ListConnections(ctx, sqlcgen.ListConnectionsParams{TenantID: f.Tenant, Kind: kind, IncludeDeleted: f.IncludeDeleted})
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: list connections: %w", err)
	}
	out := make([]filament.Connection, len(rows))
	for i, row := range rows {
		c, err := connectionFromRow(row.ConnectionID, row.TenantID, row.Kind, row.Name, row.Provider, row.Config, row.SecretRefs, row.Version, row.DeletedAt)
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

// DeleteConnection soft-deletes the connection with the given ID; deleting a
// missing ID is a no-op. The name is freed for reuse by the partial unique index.
func (s *Store) DeleteConnection(ctx context.Context, id string) error {
	if err := s.q.DeleteConnection(ctx, id); err != nil {
		return fmt.Errorf("datastore/postgres: delete connection: %w", err)
	}
	return nil
}
