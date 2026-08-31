// Package memory implements filament.DataStore and filament.ScheduleStore with
// in-process maps, mirroring the semantics of datastore/postgres. Nothing
// survives the process; it is the default store for tests and local runs.
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// ErrNotFound is returned (wrapped) when a requested run or checkpoint does not
// exist. It aliases filament.ErrNotFound so callers branch with errors.Is on the
// shared sentinel regardless of which DataStore impl they hold.
var ErrNotFound = filament.ErrNotFound

// deletedNameTimestamp renders the delete time stamped onto a soft-deleted
// pipeline or connection name, keeping the name unique if the row is ever
// restored into a partial unique index. Fixed-width milliseconds, matching the
// postgres to_char pattern in queries/{pipelines,connections}.sql —
// time.RFC3339 has no fractional seconds and RFC3339Nano trims trailing zeros.
const deletedNameTimestamp = "2006-01-02T15:04:05.000Z"

// stampDeletedName appends the delete marker the postgres store writes in SQL.
func stampDeletedName(name string, at time.Time) string {
	return fmt.Sprintf("%s__deleted__%s", name, at.UTC().Format(deletedNameTimestamp))
}

// Store is an in-memory DataStore.
type Store struct {
	mu                  sync.RWMutex
	runs                map[filament.RunID]filament.RunState
	resources           map[filament.RunID]map[string]filament.ResourceState // run → resource → state
	checkpoints         map[ckey]filament.Checkpoint
	resourceCheckpoints map[filament.ResourceCheckpointKey]filament.ResourceCheckpointState
	seen                map[dkey]uint64 // dedup high-water mark per (tenant, run)
	connections         map[string]filament.Connection
	deletedConnections  map[string]filament.Connection
	pipelines           map[string]*ingestionv1.Pipeline
	deletedPipelines    map[string]*ingestionv1.Pipeline
	pipelineVersions    map[string]map[int64]*ingestionv1.PipelineVersion
	schedules           map[filament.ScheduleID]filament.ScheduleState
	scheduleClaims      map[filament.ScheduleID]time.Time
}

type ckey struct {
	run      filament.RunID
	resource string
}

type dkey struct {
	tenant string
	run    filament.RunID
}

// New returns a ready-to-use in-memory store.
func New() *Store {
	return &Store{
		runs:                map[filament.RunID]filament.RunState{},
		resources:           map[filament.RunID]map[string]filament.ResourceState{},
		checkpoints:         map[ckey]filament.Checkpoint{},
		resourceCheckpoints: map[filament.ResourceCheckpointKey]filament.ResourceCheckpointState{},
		seen:                map[dkey]uint64{},
		connections:         map[string]filament.Connection{},
		deletedConnections:  map[string]filament.Connection{},
		pipelines:           map[string]*ingestionv1.Pipeline{},
		deletedPipelines:    map[string]*ingestionv1.Pipeline{},
		pipelineVersions:    map[string]map[int64]*ingestionv1.PipelineVersion{},
		schedules:           map[filament.ScheduleID]filament.ScheduleState{},
		scheduleClaims:      map[filament.ScheduleID]time.Time{},
	}
}

var (
	_ filament.DataStore     = (*Store)(nil)
	_ filament.ScheduleStore = (*Store)(nil)
)

// Name identifies this store implementation.
func (s *Store) Name() string { return "memory" }

// Ping reports the store reachable; memory is always up.
func (s *Store) Ping(context.Context) error { return nil }

// EnsureTenant is a no-op; the in-memory store keeps no tenant rows.
func (s *Store) EnsureTenant(context.Context, filament.TenantID, string) error { return nil }

// ResolveTenant keeps no tenant rows; the external id stands in for
// filament's own so callers still get a stable, non-empty id.
func (s *Store) ResolveTenant(_ context.Context, externalID, _ string) (filament.TenantID, error) {
	return filament.TenantID(externalID), nil
}

// EnsureUser keeps no user rows either; the external id stands in for
// filament's own so callers still get a stable, non-empty id.
func (s *Store) EnsureUser(_ context.Context, _ filament.TenantID, externalID string) (filament.UserID, error) {
	return filament.UserID(externalID), nil
}

// SaveRun stores the run record. Any Resources carried on it are seeded into the
// resource index (keyed by run); the stored run keeps no Resources slice — the
// index is the single source of truth, reattached on read.
func (s *Store) SaveRun(ctx context.Context, r filament.RunState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveRunLocked(r)
}

// CreateRun inserts the run or promotes a pre-created RunScheduled row; a row
// that has progressed past RunScheduled is left untouched.
func (s *Store) CreateRun(ctx context.Context, r filament.RunState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.runs[r.Run]; ok && existing.Status != filament.RunScheduled {
		return fmt.Errorf("create run %q: %w", r.Run, filament.ErrVersionConflict)
	}
	return s.saveRunLocked(r)
}

func (s *Store) saveRunLocked(r filament.RunState) error {
	if activeCheckpointRun(r) {
		for id, existing := range s.runs {
			if id != r.Run && activeCheckpointRun(existing) && sameCheckpointRoute(existing.Request, r.Request) {
				return fmt.Errorf("checkpoint route %q already has active run %q", r.Request.CheckpointRoute, id)
			}
		}
	}
	for _, rs := range r.Resources {
		rs.Run = r.Run
		s.putResourceLocked(rs)
	}
	r.Resources = nil
	r = mergeRunTimes(s.runs[r.Run], r)
	s.runs[r.Run] = r
	// A pre-created scheduled run hasn't happened yet — it must not become the
	// pipeline's last run.
	if r.Status == filament.RunScheduled {
		return nil
	}
	return nil
}

// mergeRunTimes applies the same first-write-wins rule postgres gets from
// coalesce(runs.col, EXCLUDED.col): a stamp already on the row survives a later
// save from a writer that does not own it. CreatedAt is stamped on insert and
// never moves; UpdatedAt is refreshed on every write.
func mergeRunTimes(prev, next filament.RunState) filament.RunState {
	now := time.Now()
	next.CreatedAt = prev.CreatedAt
	if next.CreatedAt.IsZero() {
		next.CreatedAt = now
	}
	next.ScheduledAt = firstSet(prev.ScheduledAt, next.ScheduledAt)
	next.RequestedAt = firstSet(prev.RequestedAt, next.RequestedAt)
	next.StartedAt = firstSet(prev.StartedAt, next.StartedAt)
	if prev.EndedAt != nil {
		next.EndedAt = prev.EndedAt
	}
	next.UpdatedAt = now
	return next
}

func firstSet(prev, next time.Time) time.Time {
	if !prev.IsZero() {
		return prev
	}
	return next
}

func activeCheckpointRun(r filament.RunState) bool {
	if r.Request.CheckpointRoute == "" {
		return false
	}
	switch r.Status {
	case filament.RunRequested, filament.RunRunning, filament.RunPaused:
		return true
	default:
		return false
	}
}

func sameCheckpointRoute(a, b filament.RunRequest) bool {
	return a.PipelineID == b.PipelineID && a.PipelineVersionID == b.PipelineVersionID && a.CheckpointRoute == b.CheckpointRoute
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

// TransitionRun atomically changes a run when its current status is admitted.
// Resume resets attempt-local lifecycle while optionally preserving counters
// whose sink output and source checkpoints survive into the next attempt.
func (s *Store) TransitionRun(
	ctx context.Context,
	id filament.RunID,
	from []filament.RunStatus,
	to filament.RunStatus,
	opts filament.RunTransitionOptions,
) (filament.RunState, error) {
	if err := ctx.Err(); err != nil {
		return filament.RunState{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.runs[id]
	if !ok {
		return filament.RunState{}, fmt.Errorf("transition run %q: %w", id, filament.ErrNotFound)
	}
	if !slices.Contains(from, state.Status) {
		return filament.RunState{}, fmt.Errorf("transition run %q from status %d: %w", id, state.Status, filament.ErrVersionConflict)
	}
	state.Status = to
	state.UpdatedAt = time.Now()
	if opts.Ended && state.EndedAt == nil {
		now := time.Now()
		state.EndedAt = &now
	}
	if opts.ResetExecution {
		state.RequestedAt = time.Now()
		state.StartedAt = time.Time{}
		state.EndedAt = nil
		state.Error = ""
		if !opts.PreserveProgress {
			state.Records = 0
			state.Bytes = 0
		}
		state.CPUSeconds = 0
		state.MemoryPeakBytes = 0
		for resource, rs := range s.resources[id] {
			rs.Status = filament.RunRequested
			if !opts.PreserveProgress {
				rs.Records = 0
				rs.Bytes = 0
			}
			rs.Error = ""
			s.resources[id][resource] = rs
		}
	}
	s.runs[id] = state
	state.Resources = s.listResourcesLocked(id)
	return state, nil
}

// DeleteRun removes the run with its resources and checkpoints; missing is a no-op.
func (s *Store) DeleteRun(ctx context.Context, id filament.RunID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteRunLocked(id)
	return nil
}

func (s *Store) deleteRunLocked(id filament.RunID) {
	delete(s.runs, id)
	delete(s.resources, id)
	for key := range s.checkpoints {
		if key.run == id {
			delete(s.checkpoints, key)
		}
	}
}

// ListRuns returns runs matching the filter, newest first by effective time:
// StartedAt, falling back to RequestedAt, ScheduledAt, then CreatedAt,
// mirroring postgres's COALESCE ordering. A run cancelled while pending keeps
// its chronological slot; a pending scheduled run sorts by its future fire
// time and so pins above finished work until promoted.
func (s *Store) ListRuns(ctx context.Context, f filament.RunFilter) ([]filament.RunState, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	s.mu.RLock()
	var out []filament.RunState
	pipelineNames := make(map[string]string)
	for id, r := range s.runs {
		if !matchRun(r, f) {
			continue
		}
		pipelineID := r.Request.PipelineID
		pipeline := s.pipelines[pipelineID]
		if pipeline == nil {
			pipeline = s.deletedPipelines[pipelineID]
		}
		pipelineName, pipelineDescription := "", ""
		if pipeline != nil {
			pipelineName, pipelineDescription = pipeline.GetName(), pipeline.GetDescription()
		}
		if !matchesSearch(f.Search, string(r.Run), pipelineID, pipelineName, pipelineDescription) {
			continue
		}
		pipelineNames[pipelineID] = pipelineName
		r.Resources = s.listResourcesLocked(id)
		out = append(out, r)
	}
	s.mu.RUnlock()

	sort.SliceStable(out, func(i, j int) bool {
		if f.SortBy == "name" {
			a := strings.ToLower(pipelineNames[out[i].Request.PipelineID])
			b := strings.ToLower(pipelineNames[out[j].Request.PipelineID])
			if a == "" && b != "" {
				return false
			}
			if a != "" && b == "" {
				return true
			}
			if a == b {
				if f.SortDescending {
					return out[i].Run > out[j].Run
				}
				return out[i].Run < out[j].Run
			}
			if f.SortDescending {
				return a > b
			}
			return a < b
		}
		if f.SortBy == "created_at" || f.SortBy == "updated_at" {
			a, b := out[i].CreatedAt, out[j].CreatedAt
			if f.SortBy == "updated_at" {
				a, b = out[i].UpdatedAt, out[j].UpdatedAt
			}
			if a.Equal(b) {
				if f.SortDescending {
					return out[i].Run > out[j].Run
				}
				return out[i].Run < out[j].Run
			}
			if f.SortDescending {
				return a.After(b)
			}
			return a.Before(b)
		}
		a, b := effectiveRunTime(out[i]), effectiveRunTime(out[j])
		if a.Equal(b) {
			if f.SortDescending {
				return out[i].Run > out[j].Run
			}
			return out[i].Run < out[j].Run
		}
		if f.SortDescending {
			return a.After(b)
		}
		return a.Before(b)
	})
	total := len(out)
	if f.Offset > 0 {
		if f.Offset >= len(out) {
			return nil, total, nil
		}
		out = out[f.Offset:]
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, total, nil
}

// effectiveRunTime is the sort stamp for the default run ordering: StartedAt,
// falling back to RequestedAt, then ScheduledAt so a pending scheduled run
// sorts by its future fire time, then CreatedAt.
func effectiveRunTime(r filament.RunState) time.Time {
	if !r.StartedAt.IsZero() {
		return r.StartedAt
	}
	if !r.RequestedAt.IsZero() {
		return r.RequestedAt
	}
	if !r.ScheduledAt.IsZero() {
		return r.ScheduledAt
	}
	return r.CreatedAt
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
	stored, err := roundTripCheckpoint(cp)
	if err != nil {
		return fmt.Errorf("datastore/memory: save checkpoint: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkpoints[ckey{run: id, resource: cp.Resource()}] = stored
	return nil
}

// roundTripCheckpoint passes the cursor through JSON so loads return the same
// shapes the postgres store produces (float64 numbers, *CheckpointData), not
// the caller's live value.
func roundTripCheckpoint(cp filament.Checkpoint) (*filament.CheckpointData, error) {
	raw, err := json.Marshal(cp.Raw())
	if err != nil {
		return nil, fmt.Errorf("marshal cursor: %w", err)
	}
	var cursor map[string]any
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return nil, fmt.Errorf("unmarshal cursor: %w", err)
	}
	return &filament.CheckpointData{ResourceName: cp.Resource(), Cursor: cursor}, nil
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

// SaveResourceCheckpoint stores cross-run progress for one pipeline route and
// resource. The checkpoint resource must agree with the key.
func (s *Store) SaveResourceCheckpoint(ctx context.Context, state filament.ResourceCheckpointState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if state.Checkpoint == nil || state.Checkpoint.Resource() != state.Key.Resource {
		return fmt.Errorf("save resource checkpoint: resource mismatch")
	}
	stored, err := roundTripCheckpoint(state.Checkpoint)
	if err != nil {
		return fmt.Errorf("datastore/memory: save resource checkpoint: %w", err)
	}
	state.Checkpoint = stored
	state.UpdatedAt = time.Now()
	s.mu.Lock()
	s.resourceCheckpoints[state.Key] = state
	s.mu.Unlock()
	return nil
}

// LoadResourceCheckpoint returns durable progress for one route/resource.
func (s *Store) LoadResourceCheckpoint(ctx context.Context, key filament.ResourceCheckpointKey) (filament.ResourceCheckpointState, error) {
	if err := ctx.Err(); err != nil {
		return filament.ResourceCheckpointState{}, err
	}
	s.mu.RLock()
	state, ok := s.resourceCheckpoints[key]
	s.mu.RUnlock()
	if !ok {
		return filament.ResourceCheckpointState{}, fmt.Errorf("load resource checkpoint %q/%q: %w", key.Route, key.Resource, ErrNotFound)
	}
	return state, nil
}

// ListResourceCheckpoints returns every durable cursor under one route,
// ordered by resource.
func (s *Store) ListResourceCheckpoints(ctx context.Context, route filament.ResourceCheckpointRoute) ([]filament.ResourceCheckpointState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	var states []filament.ResourceCheckpointState
	for key, state := range s.resourceCheckpoints {
		if key.PipelineID == route.PipelineID && key.PipelineVersionID == route.PipelineVersionID && key.Route == route.Route {
			states = append(states, state)
		}
	}
	s.mu.RUnlock()
	slices.SortFunc(states, func(a, b filament.ResourceCheckpointState) int {
		return strings.Compare(a.Key.Resource, b.Key.Resource)
	})
	return states, nil
}

// DeleteResourceCheckpoint clears durable progress so the next run starts a
// fresh backfill.
func (s *Store) DeleteResourceCheckpoint(ctx context.Context, key filament.ResourceCheckpointKey) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.resourceCheckpoints, key)
	s.mu.Unlock()
	return nil
}

// DedupSeen reports whether seq has already been applied for (tenant, run),
// advancing the run's high-water mark on the first call to see it — the same
// contract as postgres's AdvanceDedupSeen: seq only ever increases per run.
func (s *Store) DedupSeen(ctx context.Context, tenant string, run filament.RunID, seq uint64) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	k := dkey{tenant: tenant, run: run}
	s.mu.Lock()
	defer s.mu.Unlock()
	if last, ok := s.seen[k]; ok && last >= seq {
		return true, nil
	}
	s.seen[k] = seq
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
	if f.PipelineID != "" && r.Request.PipelineID != f.PipelineID {
		return false
	}
	if f.PipelineVersionID != nil && r.Request.PipelineVersionID != *f.PipelineVersionID {
		return false
	}
	if f.Schedule != "" && r.ScheduleID != f.Schedule {
		return false
	}
	if !f.Since.IsZero() && r.StartedAt.Before(f.Since) {
		return false
	}
	if !f.Until.IsZero() && (r.StartedAt.IsZero() || !r.StartedAt.Before(f.Until)) {
		return false
	}
	if !f.UpdatedBefore.IsZero() && !r.UpdatedAt.Before(f.UpdatedBefore) {
		return false
	}
	if len(f.Status) > 0 && !slices.Contains(f.Status, r.Status) {
		return false
	}
	return true
}
