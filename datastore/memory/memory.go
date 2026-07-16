// Package memory implements the filament.DataStore interface on PostgreSQL (default).
package memory

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"sync"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// ErrNotFound is returned (wrapped) when a requested run or checkpoint does not
// exist. It aliases filament.ErrNotFound so callers branch with errors.Is on the
// shared sentinel regardless of which DataStore impl they hold.
var ErrNotFound = filament.ErrNotFound

// Store is an in-memory DataStore.
type Store struct {
	mu          sync.RWMutex
	runs        map[filament.RunID]filament.RunState
	resources   map[filament.RunID]map[string]filament.ResourceState // run → resource → state
	checkpoints map[ckey]filament.Checkpoint
	seen        map[dkey]struct{} // dedup keys already applied
	connections map[string]filament.Connection
	pipelines   map[string]*ingestionv1.Pipeline
}

type ckey struct {
	run      filament.RunID
	resource string
}

type dkey struct {
	tenant string
	run    filament.RunID
	seq    uint64
}

// New returns a ready-to-use in-memory store.
func New() *Store {
	return &Store{
		runs:        map[filament.RunID]filament.RunState{},
		resources:   map[filament.RunID]map[string]filament.ResourceState{},
		checkpoints: map[ckey]filament.Checkpoint{},
		seen:        map[dkey]struct{}{},
		connections: map[string]filament.Connection{},
		pipelines:   map[string]*ingestionv1.Pipeline{},
	}
}

var _ filament.DataStore = (*Store)(nil)

// Name identifies this store implementation.
func (s *Store) Name() string { return "memory" }

// SaveRun stores the run record. Any Resources carried on it are seeded into the
// resource index (keyed by run); the stored run keeps no Resources slice — the
// index is the single source of truth, reattached on read.
func (s *Store) SaveRun(ctx context.Context, r filament.RunState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rs := range r.Resources {
		rs.Run = r.Run
		s.putResourceLocked(rs)
	}
	r.Resources = nil
	s.runs[r.Run] = r
	return nil
}

// LoadRun returns the run with its current resource states reattached.
func (s *Store) LoadRun(ctx context.Context, id filament.RunID) (filament.RunState, error) {
	if err := ctx.Err(); err != nil {
		return filament.RunState{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.runs[id]
	if !ok {
		return filament.RunState{}, fmt.Errorf("load run %q: %w", id, ErrNotFound)
	}
	r.Resources = s.listResourcesLocked(id)
	return r, nil
}

// ListRuns returns runs matching the filter, sorted by StartedAt then Run.
func (s *Store) ListRuns(ctx context.Context, f filament.RunFilter) ([]filament.RunState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	var out []filament.RunState
	for id, r := range s.runs {
		if !matchRun(r, f) {
			continue
		}
		r.Resources = s.listResourcesLocked(id)
		out = append(out, r)
	}
	s.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		if out[i].StartedAt.Equal(out[j].StartedAt) {
			return out[i].Run < out[j].Run
		}
		return out[i].StartedAt.Before(out[j].StartedAt)
	})
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// UpsertResource records (or replaces) a resource's state under its run.
func (s *Store) UpsertResource(ctx context.Context, rs filament.ResourceState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putResourceLocked(rs)
	return nil
}

// ListResources returns a run's resource states, sorted by name.
func (s *Store) ListResources(ctx context.Context, id filament.RunID) ([]filament.ResourceState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listResourcesLocked(id), nil
}

// SaveCheckpoint stores a resumable cursor keyed by (run, resource).
func (s *Store) SaveCheckpoint(ctx context.Context, id filament.RunID, cp filament.Checkpoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkpoints[ckey{run: id, resource: cp.Resource()}] = cp
	return nil
}

// LoadCheckpoint returns the saved cursor for (run, resource), or ErrNotFound.
func (s *Store) LoadCheckpoint(ctx context.Context, id filament.RunID, resource string) (filament.Checkpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp, ok := s.checkpoints[ckey{run: id, resource: resource}]
	if !ok {
		return nil, fmt.Errorf("load checkpoint %q/%q: %w", id, resource, ErrNotFound)
	}
	return cp, nil
}

// DedupSeen reports whether (tenant, run, seq) was already applied, marking it
// seen on the first call. Lets consumers make fact application idempotent.
func (s *Store) DedupSeen(ctx context.Context, tenant string, run filament.RunID, seq uint64) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	k := dkey{tenant: tenant, run: run, seq: seq}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.seen[k]; ok {
		return true, nil
	}
	s.seen[k] = struct{}{}
	return false, nil
}

// ── helpers (lock held by caller) ───────────────────────────────────────────

func (s *Store) putResourceLocked(rs filament.ResourceState) {
	m := s.resources[rs.Run]
	if m == nil {
		m = map[string]filament.ResourceState{}
		s.resources[rs.Run] = m
	}
	m[rs.Resource] = rs
}

func (s *Store) listResourcesLocked(id filament.RunID) []filament.ResourceState {
	m := s.resources[id]
	if len(m) == 0 {
		return nil
	}
	out := make([]filament.ResourceState, 0, len(m))
	for _, rs := range m {
		out = append(out, rs)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Resource < out[j].Resource })
	return out
}

func matchRun(r filament.RunState, f filament.RunFilter) bool {
	if f.Tenant != "" && r.Tenant != f.Tenant {
		return false
	}
	if f.Schedule != "" && r.ScheduleID != f.Schedule {
		return false
	}
	if !f.Since.IsZero() && r.StartedAt.Before(f.Since) {
		return false
	}
	if len(f.Status) > 0 && !slices.Contains(f.Status, r.Status) {
		return false
	}
	if f.Source != "" &&
		r.Request.Source.ConfigRef != f.Source &&
		r.Request.Source.Provider != f.Source {
		return false
	}
	return true
}
