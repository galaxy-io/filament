package memory

import (
	"context"
	"fmt"
	"slices"
	"strings"

	ingestion "github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"google.golang.org/protobuf/proto"
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
	next.Version = 1
	s.pipelines[next.Id] = clonePipeline(next)
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
		return nil, fmt.Errorf("pipeline %q: %w", p.GetId(), ingestion.ErrNotFound)
	}
	if stored.GetVersion() != p.GetVersion() {
		return nil, fmt.Errorf("pipeline %q: %w", p.GetId(), ingestion.ErrVersionConflict)
	}
	next := clonePipeline(p)
	next.Version++
	s.pipelines[next.Id] = clonePipeline(next)
	return next, nil
}

func (s *Store) LoadPipeline(ctx context.Context, id string) (*ingestionv1.Pipeline, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.pipelines[id]
	if !ok {
		return nil, fmt.Errorf("pipeline %q: %w", id, ingestion.ErrNotFound)
	}
	return clonePipeline(p), nil
}

func (s *Store) ListPipelines(ctx context.Context, tenant string) ([]*ingestionv1.Pipeline, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*ingestionv1.Pipeline
	for _, p := range s.pipelines {
		if tenant == "" || p.GetTenant() == tenant {
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
	delete(s.pipelines, id)
	return nil
}

func clonePipeline(p *ingestionv1.Pipeline) *ingestionv1.Pipeline {
	if p == nil {
		return nil
	}
	return proto.Clone(p).(*ingestionv1.Pipeline)
}
