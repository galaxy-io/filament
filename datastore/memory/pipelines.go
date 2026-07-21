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
