package memory

import (
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
	if !exists {
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
func (s *Store) LoadConnection(ctx context.Context, id string) (filament.Connection, error) {
	if err := ctx.Err(); err != nil {
		return filament.Connection{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, exists := s.connections[id]
	if !exists {
		c, exists = s.deletedConnections[id]
	}
	if !exists {
		return filament.Connection{}, fmt.Errorf("connection %q: %w", id, filament.ErrNotFound)
	}
	return cloneConnection(c), nil
}

// ListConnections returns connections matching the filter, sorted by ID.
func (s *Store) ListConnections(ctx context.Context, f filament.ConnectionFilter) ([]filament.Connection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []filament.Connection
	appendMatching := func(connections map[string]filament.Connection) {
		for _, c := range connections {
			if f.Tenant != "" && c.Tenant != f.Tenant {
				continue
			}
			if f.Kind != filament.ConnectorKindUnspecified && c.Kind != f.Kind {
				continue
			}
			out = append(out, cloneConnection(c))
		}
	}
	appendMatching(s.connections)
	if f.IncludeDeleted {
		appendMatching(s.deletedConnections)
	}
	slices.SortFunc(out, func(a, b filament.Connection) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

// DeleteConnection soft-deletes the connection with the given ID; deleting a
// missing ID is a no-op. The name is stamped with the delete time to match the
// postgres store, which mangles it so a restored row can't collide in the
// partial unique index.
func (s *Store) DeleteConnection(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, exists := s.connections[id]; exists {
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
