package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite/sqlcgen"
)

// SaveRun upserts the run row and reattaches any carried Resources into
// run_resource_states, mirroring the store contract shared with postgres.
func (s *Store) SaveRun(ctx context.Context, r filament.RunState) error {
	req, err := json.Marshal(r.Request)
	if err != nil {
		return fmt.Errorf("datastore/sqlite: marshal request: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("datastore/sqlite: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)

	rows, err := q.SaveRun(ctx, sqlcgen.SaveRunParams{
		RunID:             string(r.Run),
		TenantID:          string(r.Tenant),
		PipelineID:        r.Request.PipelineID,
		PipelineVersionID: r.Request.PipelineVersionID,
		ScheduleID:        string(r.ScheduleID),
		Status:            int64(r.Status),
		Request:           string(req),
		Records:           r.Records,
		Bytes:             r.Bytes,
		ScheduledAt:       nullMillis(r.ScheduledAt),
		RequestedAt:       nullMillis(r.RequestedAt),
		StartedAt:         nullMillis(r.StartedAt),
		EndedAt:           nullMillisPtr(r.EndedAt),
		Error:             r.Error,
		CpuSeconds:        r.CPUSeconds,
		MemoryPeakBytes:   r.MemoryPeakBytes,
		CreatedAt:         nowMillis(),
		UpdatedAt:         nowMillis(),
	})
	if err != nil {
		return fmt.Errorf("datastore/sqlite: save run: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("save run %q: %w", r.Run, filament.ErrNotFound)
	}
	if err := upsertResources(ctx, q, r); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("datastore/sqlite: commit: %w", err)
	}
	return nil
}

// CreateRun inserts the run or promotes a pre-created RunScheduled row; a row
// that has progressed past RunScheduled is left untouched and reported as a
// conflict.
func (s *Store) CreateRun(ctx context.Context, r filament.RunState) error {
	req, err := json.Marshal(r.Request)
	if err != nil {
		return fmt.Errorf("datastore/sqlite: marshal request: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("datastore/sqlite: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)

	// Promote a pre-created RunScheduled row first; when no row matches,
	// insert. DO NOTHING on the insert means an existing row past
	// RunScheduled, which is the conflict the promote guard protects.
	now := nowMillis()
	promoted, err := q.PromoteScheduledRun(ctx, sqlcgen.PromoteScheduledRunParams{
		TenantID: string(r.Tenant), RunID: string(r.Run), FromStatus: int64(filament.RunScheduled),
		PipelineID:        r.Request.PipelineID,
		PipelineVersionID: r.Request.PipelineVersionID,
		ScheduleID:        string(r.ScheduleID),
		Status:            int64(r.Status),
		Request:           string(req),
		Records:           r.Records,
		Bytes:             r.Bytes,
		ScheduledAt:       nullMillis(r.ScheduledAt),
		RequestedAt:       nullMillis(r.RequestedAt),
		StartedAt:         nullMillis(r.StartedAt),
		EndedAt:           nullMillisPtr(r.EndedAt),
		Error:             r.Error,
		CpuSeconds:        r.CPUSeconds,
		MemoryPeakBytes:   r.MemoryPeakBytes,
		UpdatedAt:         now,
	})
	if err != nil {
		return fmt.Errorf("datastore/sqlite: promote run: %w", err)
	}
	if promoted == 0 {
		inserted, err := q.InsertRun(ctx, sqlcgen.InsertRunParams{
			RunID:             string(r.Run),
			TenantID:          string(r.Tenant),
			PipelineID:        r.Request.PipelineID,
			PipelineVersionID: r.Request.PipelineVersionID,
			ScheduleID:        string(r.ScheduleID),
			Status:            int64(r.Status),
			Request:           string(req),
			Records:           r.Records,
			Bytes:             r.Bytes,
			ScheduledAt:       nullMillis(r.ScheduledAt),
			RequestedAt:       nullMillis(r.RequestedAt),
			StartedAt:         nullMillis(r.StartedAt),
			EndedAt:           nullMillisPtr(r.EndedAt),
			Error:             r.Error,
			CpuSeconds:        r.CPUSeconds,
			MemoryPeakBytes:   r.MemoryPeakBytes,
			CreatedAt:         now,
			UpdatedAt:         now,
		})
		if err != nil {
			return fmt.Errorf("datastore/sqlite: create run: %w", err)
		}
		if inserted == 0 {
			return fmt.Errorf("create run %q: %w", r.Run, filament.ErrVersionConflict)
		}
	}
	if err := upsertResources(ctx, q, r); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("datastore/sqlite: commit: %w", err)
	}
	return nil
}

func upsertResources(ctx context.Context, q *sqlcgen.Queries, r filament.RunState) error {
	for _, rs := range r.Resources {
		rs.Run = r.Run
		rs.Tenant = r.Tenant
		if err := upsertResource(ctx, q, rs); err != nil {
			return err
		}
	}
	return nil
}

// TransitionRun serializes lifecycle commands on the run row and updates it
// only when the current status belongs to from. The single-writer connection
// provides the serialization the postgres store expresses with FOR UPDATE.
func (s *Store) TransitionRun(
	ctx context.Context,
	tenant filament.TenantID,
	id filament.RunID,
	from []filament.RunStatus,
	to filament.RunStatus,
	opts filament.RunTransitionOptions,
) (filament.RunState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return filament.RunState{}, fmt.Errorf("datastore/sqlite: begin run transition: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)

	current, err := q.GetRunStatus(ctx, sqlcgen.GetRunStatusParams{TenantID: string(tenant), RunID: string(id)})
	if errors.Is(err, sql.ErrNoRows) {
		return filament.RunState{}, fmt.Errorf("transition run %q: %w", id, filament.ErrNotFound)
	}
	if err != nil {
		return filament.RunState{}, fmt.Errorf("datastore/sqlite: lock run transition: %w", err)
	}
	if !slices.Contains(from, filament.RunStatus(current)) {
		return filament.RunState{}, fmt.Errorf("transition run %q from status %d: %w", id, current, filament.ErrVersionConflict)
	}
	now := nowMillis()
	if opts.ResetExecution {
		if err := q.ResetRunExecution(ctx, sqlcgen.ResetRunExecutionParams{
			TenantID: string(tenant), RunID: string(id), Status: int64(to),
			PreserveProgress: opts.PreserveProgress,
			RequestedAt:      sql.NullInt64{Int64: now, Valid: true}, UpdatedAt: now,
		}); err != nil {
			return filament.RunState{}, fmt.Errorf("datastore/sqlite: reset run transition: %w", err)
		}
		if err := q.ResetRunResources(ctx, sqlcgen.ResetRunResourcesParams{
			TenantID: string(tenant), RunID: string(id), Status: int64(filament.RunRequested),
			PreserveProgress: opts.PreserveProgress, UpdatedAt: now,
		}); err != nil {
			return filament.RunState{}, fmt.Errorf("datastore/sqlite: reset resource transition: %w", err)
		}
	} else {
		if err := q.TransitionRun(ctx, sqlcgen.TransitionRunParams{
			TenantID: string(tenant), RunID: string(id), Status: int64(to),
			StampEnded: opts.Ended, EndedNow: sql.NullInt64{Int64: now, Valid: true}, UpdatedAt: now,
		}); err != nil {
			return filament.RunState{}, fmt.Errorf("datastore/sqlite: run transition: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return filament.RunState{}, fmt.Errorf("datastore/sqlite: commit run transition: %w", err)
	}
	return s.LoadRun(ctx, tenant, id)
}

// DeleteRun removes the run; resources, checkpoints, and dedup rows cascade.
// Missing is a no-op.
func (s *Store) DeleteRun(ctx context.Context, tenant filament.TenantID, id filament.RunID) error {
	if err := s.q.DeleteRun(ctx, sqlcgen.DeleteRunParams{TenantID: string(tenant), RunID: string(id)}); err != nil {
		return fmt.Errorf("datastore/sqlite: delete run: %w", err)
	}
	return nil
}

// LoadRun returns the run with its current resource states reattached.
func (s *Store) LoadRun(ctx context.Context, tenant filament.TenantID, id filament.RunID) (filament.RunState, error) {
	row, err := s.q.LoadRun(ctx, sqlcgen.LoadRunParams{TenantID: string(tenant), RunID: string(id)})
	if errors.Is(err, sql.ErrNoRows) {
		return filament.RunState{}, fmt.Errorf("load run %q: %w", id, filament.ErrNotFound)
	}
	if err != nil {
		return filament.RunState{}, fmt.Errorf("datastore/sqlite: load run: %w", err)
	}
	r, err := runFromRow(row.ID, row.TenantID, row.ScheduleID, row.Status, row.Request, row.Records, row.Bytes,
		row.CreatedAt, row.ScheduledAt, row.RequestedAt, row.StartedAt, row.EndedAt, row.UpdatedAt,
		row.Error, row.CpuSeconds, row.MemoryPeakBytes)
	if err != nil {
		return filament.RunState{}, err
	}
	r.Resources, err = s.ListResources(ctx, tenant, id)
	if err != nil {
		return filament.RunState{}, err
	}
	return r, nil
}

// ListRuns returns runs matching the filter, newest effective time first, each
// with resource states attached, plus the pre-page total. The query is built
// dynamically, like the postgres store's, because the filter shape doesn't fit
// a static sqlc statement. Effective time is StartedAt, falling back to
// RequestedAt, ScheduledAt, then CreatedAt.
func (s *Store) ListRuns(ctx context.Context, f filament.RunFilter) ([]filament.RunState, int, error) {
	query, args := listRunsQuery(f)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("datastore/sqlite: list runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []filament.RunState
	var total int
	for rows.Next() {
		var (
			runID, tenant, scheduleID, errMsg, req                    string
			status, records, bytes, memoryPeakBytes, created, updated int64
			scheduled, requested, started, ended                      sql.NullInt64
			cpuSeconds                                                float64
		)
		if err := rows.Scan(&runID, &tenant, &scheduleID, &status, &req, &records, &bytes,
			&created, &scheduled, &requested, &started, &ended, &updated,
			&errMsg, &cpuSeconds, &memoryPeakBytes, &total); err != nil {
			return nil, 0, fmt.Errorf("datastore/sqlite: scan run: %w", err)
		}
		r, err := runFromRow(runID, tenant, scheduleID, status, req, records, bytes,
			created, scheduled, requested, started, ended, updated, errMsg, cpuSeconds, memoryPeakBytes)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("datastore/sqlite: list runs: %w", err)
	}
	for i := range out {
		out[i].Resources, err = s.ListResources(ctx, out[i].Tenant, out[i].Run)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

func listRunsQuery(f filament.RunFilter) (string, []any) {
	q := `SELECT r.id, r.tenant_id, coalesce(r.schedule_id, ''), r.status, r.request, r.records, r.bytes, r.created_at, r.scheduled_at, r.requested_at, r.started_at, r.ended_at, r.updated_at, coalesce(r.error, ''), r.cpu_seconds, r.memory_peak_bytes, count(*) OVER ()
	      FROM runs r LEFT JOIN pipelines p ON p.tenant_id = r.tenant_id AND p.id = r.pipeline_id WHERE 1=1`
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("?%d", len(args))
	}
	if f.Tenant != "" {
		q += " AND r.tenant_id = " + arg(string(f.Tenant))
	}
	if f.PipelineID != "" {
		q += " AND r.pipeline_id = " + arg(f.PipelineID)
	}
	if f.PipelineVersionID != nil {
		q += " AND r.pipeline_version_id = " + arg(*f.PipelineVersionID)
	}
	if f.Schedule != "" {
		q += " AND r.schedule_id = " + arg(string(f.Schedule))
	}
	if !f.Since.IsZero() {
		q += " AND r.started_at >= " + arg(f.Since.UnixMilli())
	}
	if !f.Until.IsZero() {
		q += " AND r.started_at < " + arg(f.Until.UnixMilli())
	}
	if !f.UpdatedBefore.IsZero() {
		q += " AND r.updated_at < " + arg(f.UpdatedBefore.UnixMilli())
	}
	if len(f.Status) > 0 {
		marks := make([]string, len(f.Status))
		for i, st := range f.Status {
			marks[i] = arg(int(st))
		}
		q += " AND r.status IN (" + strings.Join(marks, ", ") + ")"
	}
	if search := strings.TrimSpace(f.Search); search != "" {
		exact := arg(search)
		sub := arg(search)
		q += " AND (r.id = " + exact + " OR r.pipeline_id = " + exact +
			" OR instr(lower(p.name), lower(" + sub + ")) > 0" +
			" OR instr(lower(p.description), lower(" + sub + ")) > 0)"
	}
	direction := "ASC"
	if f.SortDescending {
		direction = "DESC"
	}
	switch f.SortBy {
	case "name":
		q += " ORDER BY lower(p.name) " + direction + " NULLS LAST, r.id " + direction
	case "created_at":
		q += " ORDER BY r.created_at " + direction + ", r.id " + direction
	case "updated_at":
		q += " ORDER BY r.updated_at " + direction + ", r.id " + direction
	default:
		q += " ORDER BY coalesce(r.started_at, r.requested_at, r.scheduled_at, r.created_at) " + direction + ", r.id " + direction
	}
	if f.Limit > 0 {
		q += " LIMIT " + arg(f.Limit)
	} else if f.Offset > 0 {
		q += " LIMIT -1"
	}
	if f.Offset > 0 {
		q += " OFFSET " + arg(f.Offset)
	}
	return q, args
}

func runFromRow(runID, tenant, scheduleID string, status int64, req string, records, bytes, created int64,
	scheduled, requested, started, ended sql.NullInt64, updated int64, errMsg string, cpuSeconds float64, memoryPeakBytes int64,
) (filament.RunState, error) {
	r := filament.RunState{
		Run: filament.RunID(runID), Tenant: filament.TenantID(tenant), ScheduleID: filament.ScheduleID(scheduleID),
		Status: filament.RunStatus(status), Records: records, Bytes: bytes, Error: errMsg,
		CPUSeconds: cpuSeconds, MemoryPeakBytes: memoryPeakBytes,
		CreatedAt: millisTime(created), ScheduledAt: derefTime(timeFrom(scheduled)),
		RequestedAt: derefTime(timeFrom(requested)), StartedAt: derefTime(timeFrom(started)),
		UpdatedAt: millisTime(updated), EndedAt: timeFrom(ended),
	}
	if err := json.Unmarshal([]byte(req), &r.Request); err != nil {
		return filament.RunState{}, fmt.Errorf("datastore/sqlite: unmarshal request: %w", err)
	}
	return r, nil
}

// UpsertResource inserts or updates one resource state row.
func (s *Store) UpsertResource(ctx context.Context, rs filament.ResourceState) error {
	return upsertResource(ctx, s.q, rs)
}

func upsertResource(ctx context.Context, q *sqlcgen.Queries, rs filament.ResourceState) error {
	now := nowMillis()
	rows, err := q.UpsertResource(ctx, sqlcgen.UpsertResourceParams{
		RunID:        string(rs.Run),
		ResourceName: rs.Resource,
		TenantID:     string(rs.Tenant),
		Status:       int64(rs.Status),
		Records:      rs.Records,
		Bytes:        rs.Bytes,
		Error:        rs.Error,
		CreatedAt:    now,
		UpdatedAt:    now,
		RunTenantID:  string(rs.Tenant),
		StateRunID:   string(rs.Run),
	})
	if err != nil {
		return fmt.Errorf("datastore/sqlite: upsert resource: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("upsert resource for run %q: %w", rs.Run, filament.ErrNotFound)
	}
	return nil
}

// ListResources returns a run's resource states ordered by name.
func (s *Store) ListResources(ctx context.Context, tenant filament.TenantID, id filament.RunID) ([]filament.ResourceState, error) {
	rows, err := s.q.ListResources(ctx, sqlcgen.ListResourcesParams{TenantID: string(tenant), RunID: string(id)})
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: list resources: %w", err)
	}
	out := make([]filament.ResourceState, len(rows))
	for i, row := range rows {
		out[i] = filament.ResourceState{
			Run:      filament.RunID(row.RunID),
			Resource: row.ResourceName,
			Tenant:   filament.TenantID(row.TenantID),
			Enabled:  true,
			Status:   filament.RunStatus(row.Status),
			Records:  row.Records,
			Bytes:    row.Bytes,
			Error:    row.Error,
		}
	}
	return out, nil
}

// SaveCheckpoint upserts a resource's cursor for the run.
func (s *Store) SaveCheckpoint(ctx context.Context, tenant filament.TenantID, id filament.RunID, cp filament.Checkpoint) error {
	cursor, err := json.Marshal(cp.Raw())
	if err != nil {
		return fmt.Errorf("datastore/sqlite: marshal checkpoint: %w", err)
	}
	now := nowMillis()
	rows, err := s.q.SaveCheckpoint(ctx, sqlcgen.SaveCheckpointParams{
		TenantID:        string(tenant),
		RunID:           string(id),
		CheckpointRunID: string(id),
		ResourceName:    cp.Resource(),
		Cursor:          string(cursor),
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		return fmt.Errorf("datastore/sqlite: save checkpoint: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("save checkpoint for run %q: %w", id, filament.ErrNotFound)
	}
	return nil
}

// LoadCheckpoint returns a resource's saved cursor for the run.
func (s *Store) LoadCheckpoint(ctx context.Context, tenant filament.TenantID, id filament.RunID, resource string) (filament.Checkpoint, error) {
	cursor, err := s.q.LoadCheckpoint(ctx, sqlcgen.LoadCheckpointParams{TenantID: string(tenant), RunID: string(id), ResourceName: resource})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("load checkpoint %q/%q: %w", id, resource, filament.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: load checkpoint: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(cursor), &raw); err != nil {
		return nil, fmt.Errorf("datastore/sqlite: unmarshal checkpoint: %w", err)
	}
	return &filament.CheckpointData{ResourceName: resource, Cursor: raw}, nil
}

// SaveResourceCheckpoint upserts durable cross-run progress for one immutable
// pipeline route and resource.
func (s *Store) SaveResourceCheckpoint(ctx context.Context, tenant filament.TenantID, state filament.ResourceCheckpointState) error {
	if state.Checkpoint == nil || state.Checkpoint.Resource() != state.Key.Resource {
		return fmt.Errorf("datastore/sqlite: save resource checkpoint: resource mismatch")
	}
	cursor, err := json.Marshal(state.Checkpoint.Raw())
	if err != nil {
		return fmt.Errorf("datastore/sqlite: marshal resource checkpoint: %w", err)
	}
	now := nowMillis()
	rows, err := s.q.SaveResourceCheckpoint(ctx, sqlcgen.SaveResourceCheckpointParams{
		TenantID:   string(tenant),
		PipelineID: state.Key.PipelineID, PipelineVersionID: state.Key.PipelineVersionID,
		RouteKey: state.Key.Route, ResourceName: state.Key.Resource,
		Cursor: string(cursor), LastRunID: string(state.Run),
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("datastore/sqlite: save resource checkpoint: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("save resource checkpoint for pipeline %q: %w", state.Key.PipelineID, filament.ErrNotFound)
	}
	return nil
}

// LoadResourceCheckpoint returns durable cross-run progress for one route and
// resource.
func (s *Store) LoadResourceCheckpoint(ctx context.Context, tenant filament.TenantID, key filament.ResourceCheckpointKey) (filament.ResourceCheckpointState, error) {
	row, err := s.q.LoadResourceCheckpoint(ctx, sqlcgen.LoadResourceCheckpointParams{
		TenantID:   string(tenant),
		PipelineID: key.PipelineID, PipelineVersionID: key.PipelineVersionID,
		RouteKey: key.Route, ResourceName: key.Resource,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return filament.ResourceCheckpointState{}, fmt.Errorf("load resource checkpoint %q/%q: %w", key.Route, key.Resource, filament.ErrNotFound)
	}
	if err != nil {
		return filament.ResourceCheckpointState{}, fmt.Errorf("datastore/sqlite: load resource checkpoint: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(row.Cursor), &raw); err != nil {
		return filament.ResourceCheckpointState{}, fmt.Errorf("datastore/sqlite: unmarshal resource checkpoint: %w", err)
	}
	return filament.ResourceCheckpointState{
		Key: key, Run: filament.RunID(row.LastRunID), UpdatedAt: millisTime(row.UpdatedAt),
		Checkpoint: &filament.CheckpointData{ResourceName: key.Resource, Cursor: raw},
	}, nil
}

// ListResourceCheckpoints returns every durable cursor under one route,
// ordered by resource.
func (s *Store) ListResourceCheckpoints(ctx context.Context, tenant filament.TenantID, route filament.ResourceCheckpointRoute) ([]filament.ResourceCheckpointState, error) {
	rows, err := s.q.ListResourceCheckpoints(ctx, sqlcgen.ListResourceCheckpointsParams{
		TenantID:   string(tenant),
		PipelineID: route.PipelineID, PipelineVersionID: route.PipelineVersionID, RouteKey: route.Route,
	})
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: list resource checkpoints: %w", err)
	}
	states := make([]filament.ResourceCheckpointState, 0, len(rows))
	for _, row := range rows {
		var raw map[string]any
		if err := json.Unmarshal([]byte(row.Cursor), &raw); err != nil {
			return nil, fmt.Errorf("datastore/sqlite: unmarshal resource checkpoint: %w", err)
		}
		key := filament.ResourceCheckpointKey{
			PipelineID: route.PipelineID, PipelineVersionID: route.PipelineVersionID,
			Route: route.Route, Resource: row.ResourceName,
		}
		states = append(states, filament.ResourceCheckpointState{
			Key: key, Run: filament.RunID(row.LastRunID), UpdatedAt: millisTime(row.UpdatedAt),
			Checkpoint: &filament.CheckpointData{ResourceName: row.ResourceName, Cursor: raw},
		})
	}
	return states, nil
}

// DeleteResourceCheckpoint resets durable progress for one route/resource.
func (s *Store) DeleteResourceCheckpoint(ctx context.Context, tenant filament.TenantID, key filament.ResourceCheckpointKey) error {
	err := s.q.DeleteResourceCheckpoint(ctx, sqlcgen.DeleteResourceCheckpointParams{
		TenantID:   string(tenant),
		PipelineID: key.PipelineID, PipelineVersionID: key.PipelineVersionID,
		RouteKey: key.Route, ResourceName: key.Resource,
	})
	if err != nil {
		return fmt.Errorf("datastore/sqlite: delete resource checkpoint: %w", err)
	}
	return nil
}

// DedupSeen reports whether seq has already been applied for (tenant, run),
// advancing the run's high-water mark on the first call to see it. This
// relies on the tracker processing each run's facts through one sequential
// pump, so seq only ever increases per run.
func (s *Store) DedupSeen(ctx context.Context, tenant string, run filament.RunID, seq uint64) (bool, error) {
	now := nowMillis()
	_, err := s.q.AdvanceDedupSeen(ctx, sqlcgen.AdvanceDedupSeenParams{
		TenantID:  tenant,
		RunID:     string(run),
		Seq:       int64(seq), //nolint:gosec // per-run batch counter, never near int64 max
		CreatedAt: now,
		UpdatedAt: now,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("datastore/sqlite: dedup seen: %w", err)
	}
	return false, nil
}
