package memory

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

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
	next := clonePipeline(p)
	s.pipelines[next.Id] = clonePipeline(next)
	return next, nil
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
	next.Id = pipelineID
	next.Version = p.GetCurrentVersionId() + 1
	next.CreatedAt = time.Now().UnixMilli()
	versions[next.Version] = clonePipelineVersion(next)
	p.CurrentVersionId = next.Version
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
	return clonePipeline(stored), nil
}

// LoadPipeline returns a pipeline by ID.
func (s *Store) LoadPipeline(ctx context.Context, id string) (*ingestionv1.Pipeline, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.pipelines[id]
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
		if p := s.pipelines[pipelineID]; p != nil {
			version = p.GetCurrentVersionId()
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
func (s *Store) ListPipelineVersions(ctx context.Context, pipelineID string) ([]*ingestionv1.PipelineVersion, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*ingestionv1.PipelineVersion
	for _, v := range s.pipelineVersions[pipelineID] {
		out = append(out, clonePipelineVersion(v))
	}
	slices.SortFunc(out, func(a, b *ingestionv1.PipelineVersion) int {
		return int(b.GetVersion() - a.GetVersion())
	})
	return out, nil
}

// ListPipelines returns pipelines, optionally filtered by tenant.
func (s *Store) ListPipelines(ctx context.Context, tenant string) ([]*ingestionv1.Pipeline, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*ingestionv1.Pipeline
	for _, p := range s.pipelines {
		if tenant == "" || p.GetTenantId() == tenant {
			out = append(out, clonePipeline(p))
		}
	}
	slices.SortFunc(out, func(a, b *ingestionv1.Pipeline) int { return strings.Compare(a.GetId(), b.GetId()) })
	return out, nil
}

// DeletePipeline removes a pipeline and all of its graph versions.
func (s *Store) DeletePipeline(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pipelineVersions, id)
	delete(s.pipelines, id)
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

func pipelineRunStatusToProto(status filament.RunStatus) ingestionv1.RunStatus {
	switch status {
	case filament.RunRequested:
		return ingestionv1.RunStatus_RUN_STATUS_REQUESTED
	case filament.RunRunning:
		return ingestionv1.RunStatus_RUN_STATUS_RUNNING
	case filament.RunCompleted:
		return ingestionv1.RunStatus_RUN_STATUS_COMPLETED
	case filament.RunFailed:
		return ingestionv1.RunStatus_RUN_STATUS_FAILED
	case filament.RunCanceled:
		return ingestionv1.RunStatus_RUN_STATUS_CANCELED
	case filament.RunPaused:
		return ingestionv1.RunStatus_RUN_STATUS_PAUSED
	case filament.RunPartial:
		return ingestionv1.RunStatus_RUN_STATUS_PARTIAL
	default:
		return ingestionv1.RunStatus_RUN_STATUS_UNSPECIFIED
	}
}
