package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

func (s *Store) CreateConnection(ctx context.Context, c ingestion.Connection) (ingestion.Connection, error) {
	if c.ID == "" {
		return ingestion.Connection{}, fmt.Errorf("datastore/postgres: connection id is required")
	}
	configJSON, refsJSON, err := marshalConnectionConfig(c)
	if err != nil {
		return ingestion.Connection{}, err
	}
	err = s.q.CreateConnection(ctx, sqlcgen.CreateConnectionParams{ConnectionID: c.ID, TenantID: c.Tenant, Kind: connectionKindToDB(c.Kind), Name: c.Name, Provider: c.Connector, Config: configJSON, SecretRefs: refsJSON})
	if err != nil {
		return ingestion.Connection{}, fmt.Errorf("datastore/postgres: create connection: %w", err)
	}
	c.Version = 1
	return c, nil
}

func (s *Store) UpdateConnection(ctx context.Context, c ingestion.Connection) (ingestion.Connection, error) {
	if c.ID == "" {
		return ingestion.Connection{}, fmt.Errorf("datastore/postgres: connection id is required")
	}
	configJSON, refsJSON, err := marshalConnectionConfig(c)
	if err != nil {
		return ingestion.Connection{}, err
	}
	newVersion, err := s.q.UpdateConnection(ctx, sqlcgen.UpdateConnectionParams{Name: c.Name, Provider: c.Connector, Config: configJSON, SecretRefs: refsJSON, ConnectionID: c.ID, ExpectedVersion: c.Version})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ingestion.Connection{}, fmt.Errorf("update connection %q at version %d: %w", c.ID, c.Version, ingestion.ErrVersionConflict)
		}
		return ingestion.Connection{}, fmt.Errorf("datastore/postgres: update connection: %w", err)
	}
	c.Version = newVersion
	return c, nil
}

func (s *Store) LoadConnection(ctx context.Context, id string) (ingestion.Connection, error) {
	row, err := s.q.GetConnection(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ingestion.Connection{}, fmt.Errorf("get connection %q: %w", id, ingestion.ErrNotFound)
		}
		return ingestion.Connection{}, fmt.Errorf("datastore/postgres: get connection: %w", err)
	}
	return connectionFromRow(row.ConnectionID, row.TenantID, row.Kind, row.Name, row.Provider, row.Config, row.SecretRefs, row.Version)
}

func (s *Store) ListConnections(ctx context.Context, f ingestion.ConnectionFilter) ([]ingestion.Connection, error) {
	kind := sqlcgen.NullConnectionKind{}
	if f.Kind != ingestion.ConnectorKindUnspecified {
		kind = sqlcgen.NullConnectionKind{ConnectionKind: connectionKindToDB(f.Kind), Valid: true}
	}
	rows, err := s.q.ListConnections(ctx, sqlcgen.ListConnectionsParams{TenantID: f.Tenant, Kind: kind})
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: list connections: %w", err)
	}
	out := make([]ingestion.Connection, len(rows))
	for i, row := range rows {
		c, err := connectionFromRow(row.ConnectionID, row.TenantID, row.Kind, row.Name, row.Provider, row.Config, row.SecretRefs, row.Version)
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

func (s *Store) DeleteConnection(ctx context.Context, id string) error {
	if err := s.q.DeleteConnection(ctx, id); err != nil {
		return fmt.Errorf("datastore/postgres: delete connection: %w", err)
	}
	return nil
}
