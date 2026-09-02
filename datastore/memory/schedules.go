package memory

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/galaxy-io/filament"
)

// SaveSchedule creates or updates a pipeline schedule.
func (s *Store) SaveSchedule(ctx context.Context, schedule filament.ScheduleState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	pipeline, ok := s.pipelines[schedule.Spec.PipelineID]
	if !ok {
		return fmt.Errorf("save schedule: pipeline %q: %w", schedule.Spec.PipelineID, ErrNotFound)
	}
	if pipeline.GetTenantId() != string(schedule.Spec.Tenant) {
		return fmt.Errorf("save schedule: pipeline tenant does not match schedule tenant")
	}
	for id, existing := range s.schedules {
		if id != schedule.ID && existing.Spec.PipelineID == schedule.Spec.PipelineID {
			return fmt.Errorf("save schedule: pipeline %q already has a schedule", schedule.Spec.PipelineID)
		}
	}
	s.schedules[schedule.ID] = schedule
	delete(s.scheduleClaims, schedule.ID)
	return nil
}

// LoadPipelineSchedule returns the schedule associated with a pipeline.
func (s *Store) LoadPipelineSchedule(ctx context.Context, tenant filament.TenantID, pipelineID string) (filament.ScheduleState, error) {
	if err := ctx.Err(); err != nil {
		return filament.ScheduleState{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, schedule := range s.schedules {
		if schedule.Spec.Tenant == tenant && schedule.Spec.PipelineID == pipelineID {
			return schedule, nil
		}
	}
	return filament.ScheduleState{}, fmt.Errorf("load pipeline schedule %q: %w", pipelineID, ErrNotFound)
}

// LoadSchedule returns a schedule by ID.
func (s *Store) LoadSchedule(ctx context.Context, tenant filament.TenantID, id filament.ScheduleID) (filament.ScheduleState, error) {
	if err := ctx.Err(); err != nil {
		return filament.ScheduleState{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	schedule, ok := s.schedules[id]
	if !ok || schedule.Spec.Tenant != tenant {
		return filament.ScheduleState{}, fmt.Errorf("load schedule %q: %w", id, ErrNotFound)
	}
	return schedule, nil
}

// ListSchedules returns schedules matching the supplied filter.
func (s *Store) ListSchedules(ctx context.Context, filter filament.ScheduleFilter) ([]filament.ScheduleState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]filament.ScheduleState, 0, len(s.schedules))
	for _, schedule := range s.schedules {
		if schedule.Spec.Tenant != filter.Tenant {
			continue
		}
		if filter.Enabled != nil && schedule.Enabled != *filter.Enabled {
			continue
		}
		out = append(out, schedule)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

// DeleteSchedule removes a schedule by ID along with its pre-created scheduled
// runs, so nothing lingers as upcoming work.
func (s *Store) DeleteSchedule(ctx context.Context, tenant filament.TenantID, id filament.ScheduleID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if schedule, ok := s.schedules[id]; !ok || schedule.Spec.Tenant != tenant {
		return nil
	}
	for runID, r := range s.runs {
		if r.ScheduleID == id && r.Status == filament.RunScheduled {
			s.deleteRunLocked(runID)
		}
	}
	delete(s.schedules, id)
	delete(s.scheduleClaims, id)
	return nil
}

// ClaimDue claims and returns schedules due to fire at or before now.
func (s *Store) ClaimDue(ctx context.Context, now time.Time, limit int) ([]filament.ScheduleState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []filament.ScheduleState
	for id, schedule := range s.schedules {
		claimedAt, claimed := s.scheduleClaims[id]
		if schedule.Enabled && schedule.NextFire != nil && !schedule.NextFire.After(now) &&
			(!claimed || claimedAt.Before(now.Add(-filament.ScheduleLeaseTTL))) {
			out = append(out, schedule)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NextFire.Before(*out[j].NextFire) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	for _, schedule := range out {
		s.scheduleClaims[schedule.ID] = now
	}
	return out, nil
}

// ReleaseScheduleClaim releases the active claim for a schedule.
func (s *Store) ReleaseScheduleClaim(ctx context.Context, tenant filament.TenantID, id filament.ScheduleID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if schedule, ok := s.schedules[id]; ok && schedule.Spec.Tenant == tenant {
		delete(s.scheduleClaims, id)
	}
	return nil
}
