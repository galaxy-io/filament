package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite/sqlcgen"
)

// Tenant is a control-plane tenant row.
type Tenant struct {
	ID        filament.TenantID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// EnsureTenant creates or updates a tenant row.
func (s *Store) EnsureTenant(ctx context.Context, id filament.TenantID, name string) error {
	if id == "" {
		return fmt.Errorf("datastore/sqlite: tenant id is required")
	}
	if err := id.Valid(); err != nil {
		return fmt.Errorf("datastore/sqlite: tenant id: %w", err)
	}
	now := nowMillis()
	if err := s.q.EnsureTenant(ctx, sqlcgen.EnsureTenantParams{TenantID: string(id), Name: name, CreatedAt: now, UpdatedAt: now}); err != nil {
		return fmt.Errorf("datastore/sqlite: ensure tenant: %w", err)
	}
	return nil
}

// ResolveTenant maps a provider organization id onto filament's tenant id,
// minting the row on first sight and refreshing the display name.
func (s *Store) ResolveTenant(ctx context.Context, externalID, name string) (filament.TenantID, error) {
	if externalID == "" {
		return "", fmt.Errorf("datastore/sqlite: tenant external id is required")
	}
	now := nowMillis()
	id, err := s.q.ResolveTenant(ctx, sqlcgen.ResolveTenantParams{
		TenantID: uuid.NewString(), ExternalID: sql.NullString{String: externalID, Valid: true},
		Name: name, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return "", fmt.Errorf("datastore/sqlite: resolve tenant: %w", err)
	}
	return filament.TenantID(id), nil
}

// LoadTenant returns a tenant row by id.
func (s *Store) LoadTenant(ctx context.Context, id filament.TenantID) (Tenant, error) {
	row, err := s.q.LoadTenant(ctx, string(id))
	if errors.Is(err, sql.ErrNoRows) {
		return Tenant{}, fmt.Errorf("load tenant %q: %w", id, filament.ErrNotFound)
	}
	if err != nil {
		return Tenant{}, fmt.Errorf("datastore/sqlite: load tenant: %w", err)
	}
	return Tenant{ID: filament.TenantID(row.ID), Name: row.Name, CreatedAt: millisTime(row.CreatedAt), UpdatedAt: millisTime(row.UpdatedAt)}, nil
}

// ListTenants returns all tenant rows ordered by id.
func (s *Store) ListTenants(ctx context.Context) ([]Tenant, error) {
	rows, err := s.q.ListTenants(ctx)
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: list tenants: %w", err)
	}
	out := make([]Tenant, len(rows))
	for i, row := range rows {
		out[i] = Tenant{ID: filament.TenantID(row.ID), Name: row.Name, CreatedAt: millisTime(row.CreatedAt), UpdatedAt: millisTime(row.UpdatedAt)}
	}
	return out, nil
}

// EnsureUser creates or touches the user row for a provider subject within a
// tenant and returns filament's id for it.
func (s *Store) EnsureUser(ctx context.Context, tenant filament.TenantID, externalID string) (filament.UserID, error) {
	if err := tenant.Valid(); err != nil {
		return "", fmt.Errorf("datastore/sqlite: tenant id: %w", err)
	}
	if externalID == "" {
		return "", fmt.Errorf("datastore/sqlite: user external id is required")
	}
	now := nowMillis()
	id, err := s.q.EnsureUser(ctx, sqlcgen.EnsureUserParams{
		UserID: uuid.NewString(), TenantID: string(tenant), ExternalID: externalID, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return "", fmt.Errorf("datastore/sqlite: ensure user: %w", err)
	}
	return filament.UserID(id), nil
}
