package memory

import (
	"context"
	"fmt"
	"slices"
	"strings"

	ingestion "github.com/galaxy-io/filament"
)

func (s *Store) CreateConnection(ctx context.Context, c ingestion.Connection) (ingestion.Connection, error) {
	if err := ctx.Err(); err != nil {
		return ingestion.Connection{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.connections[c.ID]; exists {
		return ingestion.Connection{}, fmt.Errorf("connection %q already exists", c.ID)
	}
	c.Version = 1
	c = cloneConnection(c)
	s.connections[c.ID] = c
	return cloneConnection(c), nil
}
func (s *Store) UpdateConnection(ctx context.Context, c ingestion.Connection) (ingestion.Connection, error) {
	if err := ctx.Err(); err != nil {
		return ingestion.Connection{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, exists := s.connections[c.ID]
	if !exists {
		return ingestion.Connection{}, fmt.Errorf("connection %q: %w", c.ID, ingestion.ErrNotFound)
	}
	if stored.Version != c.Version {
		return ingestion.Connection{}, fmt.Errorf("connection %q version conflict", c.ID)
	}
	c.Version++
	c = cloneConnection(c)
	s.connections[c.ID] = c
	return cloneConnection(c), nil
}
func (s *Store) LoadConnection(ctx context.Context, id string) (ingestion.Connection, error) {
	if err := ctx.Err(); err != nil {
		return ingestion.Connection{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, exists := s.connections[id]
	if !exists {
		return ingestion.Connection{}, fmt.Errorf("connection %q: %w", id, ingestion.ErrNotFound)
	}
	return cloneConnection(c), nil
}
func (s *Store) ListConnections(ctx context.Context, f ingestion.ConnectionFilter) ([]ingestion.Connection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ingestion.Connection
	for _, c := range s.connections {
		if f.Tenant != "" && c.Tenant != f.Tenant {
			continue
		}
		if f.Kind != ingestion.ConnectorKindUnspecified && c.Kind != f.Kind {
			continue
		}
		out = append(out, cloneConnection(c))
	}
	slices.SortFunc(out, func(a, b ingestion.Connection) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}
func (s *Store) DeleteConnection(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.connections, id)
	return nil
}

func cloneConnection(c ingestion.Connection) ingestion.Connection {
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
