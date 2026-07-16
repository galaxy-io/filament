package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

// DefaultTenantID is the tenant seeded by migrations and used by POC commands.
const DefaultTenantID = "t1"

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
		return fmt.Errorf("datastore/postgres: tenant id is required")
	}
	if err := id.Valid(); err != nil {
		return fmt.Errorf("datastore/postgres: tenant id: %w", err)
	}
	if err := s.q.EnsureTenant(ctx, sqlcgen.EnsureTenantParams{TenantID: string(id), Name: name}); err != nil {
		return fmt.Errorf("datastore/postgres: ensure tenant: %w", err)
	}
	return nil
}

// LoadTenant returns a tenant row by id.
func (s *Store) LoadTenant(ctx context.Context, id filament.TenantID) (Tenant, error) {
	row, err := s.q.LoadTenant(ctx, string(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tenant{}, fmt.Errorf("load tenant %q: %w", id, filament.ErrNotFound)
		}
		return Tenant{}, fmt.Errorf("datastore/postgres: load tenant: %w", err)
	}
	return tenantFromRow(row.TenantID, row.Name, fromTimestamptz(row.CreatedAt), fromTimestamptz(row.UpdatedAt)), nil
}

// ListTenants returns all tenant rows ordered by id.
func (s *Store) ListTenants(ctx context.Context) ([]Tenant, error) {
	rows, err := s.q.ListTenants(ctx)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: list tenants: %w", err)
	}
	out := make([]Tenant, len(rows))
	for i, row := range rows {
		out[i] = tenantFromRow(row.TenantID, row.Name, fromTimestamptz(row.CreatedAt), fromTimestamptz(row.UpdatedAt))
	}
	return out, nil
}

func tenantFromRow(id, name string, created, updated *time.Time) Tenant {
	t := Tenant{ID: filament.TenantID(id), Name: name}
	if created != nil {
		t.CreatedAt = *created
	}
	if updated != nil {
		t.UpdatedAt = *updated
	}
	return t
}
