package memory

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// CreatePipeline stores a new pipeline.
func (s *Store) CreatePipeline(ctx context.Context, p *ingestionv1.Pipeline) (*ingestionv1.Pipeline, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pipelines[p.GetId()]; ok {
		return nil, fmt.Errorf("pipeline %q already exists", p.GetId())
	}
	delete(s.deletedPipelines, p.GetId())
	next := clonePipeline(p)
	next.CreatedAt = time.Now().UnixMilli()
	next.UpdatedAt = next.CreatedAt
	s.pipelines[next.Id] = clonePipeline(next)
	return next, nil
}

// CreatePipelineWithSchedule stores a pipeline and optional schedule atomically.
func (s *Store) CreatePipelineWithSchedule(ctx context.Context, p *ingestionv1.Pipeline, schedule *filament.ScheduleState) (*ingestionv1.Pipeline, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.pipelines[p.GetId()]; exists {
		return nil, fmt.Errorf("create pipeline %q: already exists", p.GetId())
	}
	delete(s.deletedPipelines, p.GetId())
	stored := clonePipeline(p)
	stored.CreatedAt = time.Now().UnixMilli()
	stored.UpdatedAt = stored.CreatedAt
	s.pipelines[p.GetId()] = stored
	s.pipelineVersions[p.GetId()] = map[int64]*ingestionv1.PipelineVersion{}
	if schedule != nil {
		if schedule.Spec.PipelineID != p.GetId() || string(schedule.Spec.Tenant) != p.GetTenantId() {
			delete(s.pipelines, p.GetId())
			delete(s.pipelineVersions, p.GetId())
			return nil, fmt.Errorf("create pipeline: schedule target does not match pipeline")
		}
		s.schedules[schedule.ID] = *schedule
	}
	return clonePipeline(stored), nil
}

// CreatePipelineVersion appends an immutable graph version to a pipeline.
func (s *Store) CreatePipelineVersion(ctx context.Context, pipelineID string, v *ingestionv1.PipelineVersion) (*ingestionv1.PipelineVersion, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pipelines[pipelineID]
	if !ok {
		return nil, fmt.Errorf("pipeline %q: %w", pipelineID, filament.ErrNotFound)
	}
	versions := s.pipelineVersions[pipelineID]
	if versions == nil {
		versions = map[int64]*ingestionv1.PipelineVersion{}
		s.pipelineVersions[pipelineID] = versions
	}
	next := clonePipelineVersion(v)
	next.Id = uuid.NewString()
	next.Version = 1
	for _, existing := range versions {
		if existing.GetVersion() >= next.Version {
			next.Version = existing.GetVersion() + 1
		}
	}
	next.CreatedAt = time.Now().UnixMilli()
	versions[next.Version] = clonePipelineVersion(next)
	p.CurrentVersion = clonePipelineVersion(next)
	return next, nil
}

// UpdatePipeline updates a pipeline's mutable metadata.
func (s *Store) UpdatePipeline(ctx context.Context, p *ingestionv1.Pipeline) (*ingestionv1.Pipeline, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.pipelines[p.GetId()]
	if !ok {
		return nil, fmt.Errorf("pipeline %q: %w", p.GetId(), filament.ErrNotFound)
	}
	stored.Name, stored.Description = p.GetName(), p.GetDescription()
	if p.GetWorkerConfiguration() != nil {
		stored.WorkerConfiguration = proto.Clone(p.GetWorkerConfiguration()).(*ingestionv1.WorkerConfiguration)
	}
	stored.UpdatedAt = time.Now().UnixMilli()
	return clonePipeline(stored), nil
}

// LoadPipeline returns a pipeline by ID, including soft-deleted ones so callers
// can still read a deleted pipeline's metadata. DeletedAt tells them apart.
func (s *Store) LoadPipeline(ctx context.Context, id string) (*ingestionv1.Pipeline, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.pipelines[id]
	if !ok {
		p, ok = s.deletedPipelines[id]
	}
	if !ok {
		return nil, fmt.Errorf("pipeline %q: %w", id, filament.ErrNotFound)
	}
	return clonePipeline(p), nil
}

// LoadPipelineVersion returns a pipeline graph version, or the current version when version is zero.
func (s *Store) LoadPipelineVersion(ctx context.Context, pipelineID string, version int64) (*ingestionv1.PipelineVersion, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if version == 0 {
		if p := s.pipelines[pipelineID]; p != nil && p.GetCurrentVersion() != nil {
			version = p.GetCurrentVersion().GetVersion()
		}
	}
	v := s.pipelineVersions[pipelineID][version]
	if v == nil {
		return nil, fmt.Errorf("pipeline %q version %d: %w", pipelineID, version, filament.ErrNotFound)
	}
	return clonePipelineVersion(v), nil
}

// ListPipelineVersions returns all of a pipeline's graph versions, newest
// first. An unknown pipeline yields an empty slice.
func (s *Store) ListPipelineVersions(ctx context.Context, f filament.PipelineVersionFilter) ([]*ingestionv1.PipelineVersion, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if f.SortBy == "" {
		f.SortBy = "version"
		f.SortDescending = true
	}
	var out []*ingestionv1.PipelineVersion
	for _, v := range s.pipelineVersions[f.PipelineID] {
		out = append(out, clonePipelineVersion(v))
	}
	slices.SortFunc(out, func(a, b *ingestionv1.PipelineVersion) int {
		var comparison int
		switch f.SortBy {
		case "created_at":
			comparison = cmp.Compare(a.GetCreatedAt(), b.GetCreatedAt())
		case "updated_at":
			comparison = cmp.Compare(a.GetUpdatedAt(), b.GetUpdatedAt())
		default:
			comparison = cmp.Compare(a.GetVersion(), b.GetVersion())
		}
		if comparison == 0 {
			comparison = strings.Compare(a.GetId(), b.GetId())
		}
		return ordered(comparison, f.SortDescending)
	})
	total := len(out)
	return pageSlice(out, f.Offset, f.Limit), total, nil
}

// ListPipelines returns pipelines matching the filter.
func (s *Store) ListPipelines(ctx context.Context, f filament.PipelineFilter) ([]*ingestionv1.Pipeline, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*ingestionv1.Pipeline
	appendMatching := func(pipelines map[string]*ingestionv1.Pipeline) {
		for _, p := range pipelines {
			if (f.Tenant == "" || p.GetTenantId() == f.Tenant) && matchesSearch(f.Search, p.GetName(), p.GetDescription()) {
				out = append(out, clonePipeline(p))
			}
		}
	}
	appendMatching(s.pipelines)
	if f.IncludeDeleted {
		appendMatching(s.deletedPipelines)
	}
	slices.SortFunc(out, func(a, b *ingestionv1.Pipeline) int {
		var comparison int
		switch f.SortBy {
		case "name":
			comparison = strings.Compare(strings.ToLower(a.GetName()), strings.ToLower(b.GetName()))
		case "created_at":
			comparison = cmp.Compare(a.GetCreatedAt(), b.GetCreatedAt())
		case "updated_at":
			comparison = cmp.Compare(a.GetUpdatedAt(), b.GetUpdatedAt())
		default:
			comparison = strings.Compare(a.GetId(), b.GetId())
		}
		if comparison == 0 {
			comparison = strings.Compare(a.GetId(), b.GetId())
		}
		return ordered(comparison, f.SortDescending)
	})
	total := len(out)
	return pageSlice(out, f.Offset, f.Limit), total, nil
}

// DeletePipeline soft-deletes a pipeline and removes its schedules and pending
// scheduled runs so the scheduler stops firing it and nothing lingers as
// upcoming work. Versions are kept so it stays readable. The name is stamped
// with the delete time to mark it in raw listings.
func (s *Store) DeletePipeline(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, exists := s.pipelines[id]; exists {
		now := time.Now()
		p.DeletedAt = now.UnixMilli()
		p.Name = stampDeletedName(p.Name, now)
		s.deletedPipelines[id] = p
		delete(s.pipelines, id)
	}
	for scheduleID, schedule := range s.schedules {
		if schedule.Spec.PipelineID == id {
			delete(s.schedules, scheduleID)
			delete(s.scheduleClaims, scheduleID)
		}
	}
	for runID, run := range s.runs {
		if run.Request.PipelineID == id && run.Status == filament.RunScheduled {
			s.deleteRunLocked(runID)
		}
	}
	return nil
}

func clonePipeline(p *ingestionv1.Pipeline) *ingestionv1.Pipeline {
	if p == nil {
		return nil
	}
	return proto.Clone(p).(*ingestionv1.Pipeline)
}

func clonePipelineVersion(v *ingestionv1.PipelineVersion) *ingestionv1.PipelineVersion {
	if v == nil {
		return nil
	}
	return proto.Clone(v).(*ingestionv1.PipelineVersion)
}
