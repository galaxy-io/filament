package memory

import (
	"context"
	"fmt"
	"slices"
	"strings"

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
	c.Version = 1
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
	c = cloneConnection(c)
	s.connections[c.ID] = c
	return cloneConnection(c), nil
}

// LoadConnection returns the connection with the given ID, or ErrNotFound.
func (s *Store) LoadConnection(ctx context.Context, id string) (filament.Connection, error) {
	if err := ctx.Err(); err != nil {
		return filament.Connection{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, exists := s.connections[id]
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
	for _, c := range s.connections {
		if f.Tenant != "" && c.Tenant != f.Tenant {
			continue
		}
		if f.Kind != filament.ConnectorKindUnspecified && c.Kind != f.Kind {
			continue
		}
		out = append(out, cloneConnection(c))
	}
	slices.SortFunc(out, func(a, b filament.Connection) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

// DeleteConnection removes the connection with the given ID; deleting a missing ID is a no-op.
func (s *Store) DeleteConnection(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.connections, id)
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
