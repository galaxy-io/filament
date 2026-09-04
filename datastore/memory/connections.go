package memory

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/galaxy-io/filament"
)

// CreateConnection stores a new connection at version 1, rejecting a duplicate ID.
func (s *Store) CreateConnection(ctx context.Context, c filament.Connection) (filament.Connection, error) {
	if err := ctx.Err(); err != nil {
		return filament.Connection{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.connections[c.ID]; exists {
		return filament.Connection{}, fmt.Errorf("connection %q already exists", c.ID)
	}
	delete(s.deletedConnections, c.ID)
	now := time.Now().UnixMilli()
	c.Version = 1
	c.CreatedAt = now
	c.UpdatedAt = now
	c = cloneConnection(c)
	s.connections[c.ID] = c
	return cloneConnection(c), nil
}

// UpdateConnection replaces a stored connection, enforcing optimistic version matching.
func (s *Store) UpdateConnection(ctx context.Context, c filament.Connection) (filament.Connection, error) {
	if err := ctx.Err(); err != nil {
		return filament.Connection{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, exists := s.connections[c.ID]
	if !exists || stored.Tenant != c.Tenant {
		return filament.Connection{}, fmt.Errorf("connection %q: %w", c.ID, filament.ErrNotFound)
	}
	if stored.Version != c.Version {
		return filament.Connection{}, fmt.Errorf("connection %q version conflict: %w", c.ID, filament.ErrVersionConflict)
	}
	c.Version++
	c.UpdatedAt = time.Now().UnixMilli()
	c = cloneConnection(c)
	s.connections[c.ID] = c
	return cloneConnection(c), nil
}

// LoadConnection returns the connection with the given ID, including
// soft-deleted ones so callers can still read a deleted connection's metadata.
// DeletedAt tells them apart.
func (s *Store) LoadConnection(ctx context.Context, tenant filament.TenantID, id string) (filament.Connection, error) {
	if err := ctx.Err(); err != nil {
		return filament.Connection{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, exists := s.connections[id]
	if !exists || c.Tenant != string(tenant) {
		c, exists = s.deletedConnections[id]
	}
	if !exists {
		return filament.Connection{}, fmt.Errorf("connection %q: %w", id, filament.ErrNotFound)
	}
	return cloneConnection(c), nil
}

// ListConnections returns one filtered, sorted page and its pre-page total.
func (s *Store) ListConnections(ctx context.Context, f filament.ConnectionFilter) ([]filament.Connection, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []filament.Connection
	appendMatching := func(connections map[string]filament.Connection) {
		for _, c := range connections {
			if c.Tenant != f.Tenant {
				continue
			}
			if f.Kind != filament.ConnectorKindUnspecified && c.Kind != f.Kind {
				continue
			}
			if !matchesSearch(f.Search, c.Name, c.Connector) {
				continue
			}
			out = append(out, cloneConnection(c))
		}
	}
	appendMatching(s.connections)
	if f.IncludeDeleted {
		appendMatching(s.deletedConnections)
	}
	slices.SortFunc(out, func(a, b filament.Connection) int {
		var comparison int
		switch f.SortBy {
		case "name":
			comparison = strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		case "connector":
			comparison = strings.Compare(strings.ToLower(a.Connector), strings.ToLower(b.Connector))
		case "created_at":
			comparison = cmp.Compare(a.CreatedAt, b.CreatedAt)
		case "updated_at":
			comparison = cmp.Compare(a.UpdatedAt, b.UpdatedAt)
		default:
			comparison = strings.Compare(a.ID, b.ID)
		}
		if comparison == 0 {
			comparison = strings.Compare(a.ID, b.ID)
		}
		return ordered(comparison, f.SortDescending)
	})
	total := len(out)
	return pageSlice(out, f.Offset, f.Limit), total, nil
}

// DeleteConnection soft-deletes the connection with the given ID; deleting a
// missing ID is a no-op. The name is stamped with the delete time to match the
// postgres store, which mangles it so a restored row can't collide in the
// partial unique index.
func (s *Store) DeleteConnection(ctx context.Context, tenant filament.TenantID, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, exists := s.connections[id]; exists && c.Tenant == string(tenant) {
		now := time.Now()
		c.DeletedAt = now.UnixMilli()
		c.Name = stampDeletedName(c.Name, now)
		s.deletedConnections[id] = c
		delete(s.connections, id)
	}
	return nil
}

func cloneConnection(c filament.Connection) filament.Connection {
	c.Config = cloneAnyMap(c.Config)
	c.SecretRefs = cloneStringMap(c.SecretRefs)
	return c
}

func cloneAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
