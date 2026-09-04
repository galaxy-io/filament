package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite/sqlcgen"
)

// SaveSchedule upserts the schedule row, releasing any ClaimDue lease.
func (s *Store) SaveSchedule(ctx context.Context, st filament.ScheduleState) error {
	return saveSchedule(ctx, s.q, st)
}

func saveSchedule(ctx context.Context, q *sqlcgen.Queries, st filament.ScheduleState) error {
	created := nullMillis(st.CreatedAt)
	if !created.Valid {
		created = sql.NullInt64{Int64: nowMillis(), Valid: true}
	}
	rows, err := q.SaveSchedule(ctx, sqlcgen.SaveScheduleParams{
		ScheduleID:         string(st.ID),
		TenantID:           string(st.Spec.Tenant),
		PipelineTenantID:   string(st.Spec.Tenant),
		PipelineID:         st.Spec.PipelineID,
		SchedulePipelineID: st.Spec.PipelineID,
		Name:               sql.NullString{String: st.Spec.Name, Valid: st.Spec.Name != ""},
		CronExpr:           st.Spec.Cron,
		Timezone:           st.Spec.Timezone,
		OverlapPolicy:      int64(st.Spec.Overlap),
		Enabled:            boolInt(st.Enabled),
		LastFiredAt:        nullMillisPtr(st.LastFired),
		NextFireAt:         nullMillisPtr(st.NextFire),
		CreatedAt:          created.Int64,
		UpdatedAt:          nowMillis(),
	})
	if err != nil {
		return fmt.Errorf("datastore/sqlite: save schedule: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("datastore/sqlite: save schedule %q: pipeline %q: %w", st.ID, st.Spec.PipelineID, filament.ErrNotFound)
	}
	return nil
}

// LoadSchedule returns one schedule by id.
func (s *Store) LoadSchedule(ctx context.Context, tenant filament.TenantID, id filament.ScheduleID) (filament.ScheduleState, error) {
	row, err := s.q.LoadSchedule(ctx, sqlcgen.LoadScheduleParams{TenantID: string(tenant), ScheduleID: string(id)})
	if errors.Is(err, sql.ErrNoRows) {
		return filament.ScheduleState{}, fmt.Errorf("load schedule %q: %w", id, filament.ErrNotFound)
	}
	if err != nil {
		return filament.ScheduleState{}, fmt.Errorf("datastore/sqlite: load schedule: %w", err)
	}
	return scheduleFromRow(row.ID, row.TenantID, row.PipelineID, row.Name, row.CronExpr, row.Timezone,
		row.OverlapPolicy, row.Enabled, row.LastFiredAt, row.NextFireAt, row.CreatedAt), nil
}

// LoadPipelineSchedule returns the schedule attached to pipelineID.
func (s *Store) LoadPipelineSchedule(ctx context.Context, tenant filament.TenantID, pipelineID string) (filament.ScheduleState, error) {
	row, err := s.q.LoadPipelineSchedule(ctx, sqlcgen.LoadPipelineScheduleParams{TenantID: string(tenant), PipelineID: pipelineID})
	if errors.Is(err, sql.ErrNoRows) {
		return filament.ScheduleState{}, fmt.Errorf("load pipeline schedule %q: %w", pipelineID, filament.ErrNotFound)
	}
	if err != nil {
		return filament.ScheduleState{}, fmt.Errorf("datastore/sqlite: load pipeline schedule: %w", err)
	}
	return scheduleFromRow(row.ID, row.TenantID, row.PipelineID, row.Name, row.CronExpr, row.Timezone,
		row.OverlapPolicy, row.Enabled, row.LastFiredAt, row.NextFireAt, row.CreatedAt), nil
}

// ListSchedules returns schedules matching the filter.
func (s *Store) ListSchedules(ctx context.Context, f filament.ScheduleFilter) ([]filament.ScheduleState, error) {
	filterEnabled := int64(-1)
	enabled := int64(0)
	if f.Enabled != nil {
		filterEnabled = boolInt(*f.Enabled)
		enabled = filterEnabled
	}
	rows, err := s.q.ListSchedules(ctx, sqlcgen.ListSchedulesParams{
		TenantID: string(f.Tenant), FilterEnabled: filterEnabled, Enabled: enabled, Lim: listLimit(f.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: list schedules: %w", err)
	}
	out := make([]filament.ScheduleState, len(rows))
	for i, row := range rows {
		out[i] = scheduleFromRow(row.ID, row.TenantID, row.PipelineID, row.Name, row.CronExpr, row.Timezone,
			row.OverlapPolicy, row.Enabled, row.LastFiredAt, row.NextFireAt, row.CreatedAt)
	}
	return out, nil
}

// DeleteSchedule removes a schedule by id along with its pre-created scheduled
// runs, so nothing lingers as upcoming work. The runs reap goes first:
// deleting the schedules row SET-NULLs runs.schedule_id, after which the rows
// are unreachable by schedule id.
func (s *Store) DeleteSchedule(ctx context.Context, tenant filament.TenantID, id filament.ScheduleID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("datastore/sqlite: begin schedule delete: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if err := q.DeleteScheduleScheduledRuns(ctx, sqlcgen.DeleteScheduleScheduledRunsParams{
		TenantID:   string(tenant),
		ScheduleID: sql.NullString{String: string(id), Valid: true},
		Status:     int64(filament.RunScheduled),
	}); err != nil {
		return fmt.Errorf("datastore/sqlite: delete schedule scheduled runs: %w", err)
	}
	if err := q.DeleteSchedule(ctx, sqlcgen.DeleteScheduleParams{TenantID: string(tenant), ScheduleID: string(id)}); err != nil {
		return fmt.Errorf("datastore/sqlite: delete schedule: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("datastore/sqlite: commit schedule delete: %w", err)
	}
	return nil
}

// ClaimDue leases up to limit enabled schedules not currently under an
// unexpired lease, stamping claimed_at in the same transaction. The
// single-writer connection keeps two schedulers from claiming the same row.
// The lease is released by the next SaveSchedule or expires after the TTL.
func (s *Store) ClaimDue(ctx context.Context, now time.Time, limit int) ([]filament.ScheduleState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)

	rows, err := q.ClaimDue(ctx, sqlcgen.ClaimDueParams{
		Now:         sql.NullInt64{Int64: now.UnixMilli(), Valid: true},
		LeaseCutoff: sql.NullInt64{Int64: now.Add(-filament.ScheduleLeaseTTL).UnixMilli(), Valid: true},
		Lim:         listLimit(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: claim due: %w", err)
	}
	out := make([]filament.ScheduleState, len(rows))
	for i, row := range rows {
		out[i] = scheduleFromRow(row.ID, row.TenantID, row.PipelineID, row.Name, row.CronExpr, row.Timezone,
			row.OverlapPolicy, row.Enabled, row.LastFiredAt, row.NextFireAt, row.CreatedAt)
		if err := q.LeaseSchedule(ctx, sqlcgen.LeaseScheduleParams{
			ClaimedAt: sql.NullInt64{Int64: now.UnixMilli(), Valid: true}, ScheduleID: row.ID,
		}); err != nil {
			return nil, fmt.Errorf("datastore/sqlite: lease claimed schedule: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("datastore/sqlite: commit claim: %w", err)
	}
	return out, nil
}

// ReleaseScheduleClaim makes a claimed occurrence eligible for retry.
func (s *Store) ReleaseScheduleClaim(ctx context.Context, tenant filament.TenantID, id filament.ScheduleID) error {
	err := s.q.ReleaseScheduleClaim(ctx, sqlcgen.ReleaseScheduleClaimParams{
		TenantID: string(tenant), ScheduleID: string(id), UpdatedAt: nowMillis(),
	})
	if err != nil {
		return fmt.Errorf("datastore/sqlite: release schedule claim: %w", err)
	}
	return nil
}

func scheduleFromRow(id, tenant, pipelineID string, name sql.NullString, cronExpr, tz string,
	overlap, enabled int64, lastFired, nextFire sql.NullInt64, createdAt int64,
) filament.ScheduleState {
	return filament.ScheduleState{
		ID: filament.ScheduleID(id),
		Spec: filament.ScheduleSpec{
			Tenant:     filament.TenantID(tenant),
			Name:       name.String,
			PipelineID: pipelineID,
			Cron:       cronExpr,
			Timezone:   tz,
			Overlap:    filament.OverlapPolicy(overlap),
			Enabled:    enabled != 0,
		},
		Enabled:   enabled != 0,
		LastFired: timeFrom(lastFired),
		NextFire:  timeFrom(nextFire),
		CreatedAt: millisTime(createdAt),
	}
}

func boolInt(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

// listLimit resolves a caller limit for SQLite's LIMIT clause: 0 means
// unlimited, which SQLite spells -1.
func listLimit(limit int) int64 {
	if limit <= 0 {
		return -1
	}
	return int64(limit)
}
