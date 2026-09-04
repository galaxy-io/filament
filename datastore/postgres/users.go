package postgres

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

// EnsureUser creates or touches the user row for a provider subject within
// a tenant and returns filament's id for it.
func (s *Store) EnsureUser(ctx context.Context, tenant filament.TenantID, externalID string) (filament.UserID, error) {
	if err := tenant.Valid(); err != nil {
		return "", fmt.Errorf("datastore/postgres: tenant id: %w", err)
	}
	if externalID == "" {
		return "", fmt.Errorf("datastore/postgres: user external id is required")
	}
	id, err := s.q.EnsureUser(ctx, sqlcgen.EnsureUserParams{TenantID: string(tenant), ExternalID: externalID})
	if err != nil {
		return "", fmt.Errorf("datastore/postgres: ensure user: %w", err)
	}
	return filament.UserID(id), nil
}
