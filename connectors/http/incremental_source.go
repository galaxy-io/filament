package httpapi

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/connectors/http/internal/atomicwatermark"
	"github.com/galaxy-io/filament/connectors/http/internal/pipeline"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// CursorColumns exposes the manifest-declared watermark as the one durable
// cursor for a resource. HTTP cursor selection is declarative rather than
// inferred: changing it requires changing the manifest contract.
func (s *Source) CursorColumns(_ context.Context, resource string) ([]filament.CursorColumn, error) {
	if s.connector == nil || s.connector.manifest == nil {
		return nil, fmt.Errorf("httpapi source: cursor columns before configure")
	}
	res, ok := s.manifestResource(s.baseResourceName(resource))
	if !ok {
		return nil, fmt.Errorf("httpapi source: unknown resource %q", resource)
	}
	if res.Incremental == nil {
		return nil, nil
	}
	field, ok := incrementalField(res)
	if !ok {
		return nil, fmt.Errorf("httpapi source: incremental %q cursor field %q is not projected", resource, res.Incremental.CursorField)
	}
	return []filament.CursorColumn{{
		SchemaField: filament.SchemaField{
			Name: field.Name, Nullable: field.Nullable,
			Logical: logicalType(field.Type), Native: field.Type,
		},
		PrimaryKey:       slices.Contains(res.PrimaryKey, field.Name),
		Eligible:         true,
		Recommended:      true,
		Rank:             1,
		Configurable:     false,
		SupportsLookback: res.Incremental.Comparator == "time" || res.Incremental.Comparator == "numeric",
		Warning:          incrementalCursorWarning(res),
	}}, nil
}

// PlanIncremental creates a watermark-only checkpoint. Pagination cursors are
// deliberately excluded: they are valid for resuming a failed page walk, but
// carrying one into the next scheduled run alongside a newer watermark can
// skip records.
func (s *Source) PlanIncremental(_ context.Context, resources []string, prev map[string]filament.Checkpoint, cursors map[string]filament.ResourceCursorConfig) (map[string]filament.Checkpoint, error) {
	if s.connector == nil || s.connector.manifest == nil {
		return nil, fmt.Errorf("httpapi source: plan incremental before configure")
	}
	planned, err := s.planResources(resources, nil)
	if err != nil {
		return nil, err
	}
	s.incrementalResources = make(map[string]manifest.IncrementalSpec, len(planned))
	lookbacks := make(map[string]int, len(planned))
	plan := make(map[string]filament.Checkpoint, len(planned))
	for _, resource := range planned {
		base := s.baseResourceName(resource)
		res, ok := s.manifestResource(base)
		if !ok {
			return nil, fmt.Errorf("httpapi source: unknown incremental resource %q", resource)
		}
		if res.Incremental == nil {
			return nil, fmt.Errorf("httpapi source: resource %q has no incremental watermark in its manifest", resource)
		}
		field, ok := incrementalField(res)
		if !ok {
			return nil, fmt.Errorf("httpapi source: incremental %q cursor field %q is not projected", resource, res.Incremental.CursorField)
		}
		config, exactConfig := cursors[resource]
		baseConfig := cursors[base]
		if !exactConfig {
			config = baseConfig
		} else if config.Field == "" {
			config.Field = baseConfig.Field
		}
		if config.Field != "" && !strings.EqualFold(config.Field, field.Name) {
			return nil, fmt.Errorf("httpapi source: incremental %q cursor is fixed by the manifest as %q, got %q", resource, field.Name, config.Field)
		}
		spec := *res.Incremental
		if config.LookbackSeconds < 0 {
			return nil, fmt.Errorf("httpapi source: incremental %q lookback must be non-negative", resource)
		}
		if config.LookbackSeconds > 0 {
			if spec.Comparator != "time" && spec.Comparator != "numeric" {
				return nil, fmt.Errorf("httpapi source: incremental %q lookback requires comparator time or numeric", resource)
			}
			spec.OverlapSeconds = int(config.LookbackSeconds)
		}
		s.incrementalResources[resource] = spec
		lookbacks[resource] = spec.OverlapSeconds

		checkpointKey := incrementalCheckpointKey(spec)
		cols := []string{checkpointKey}
		types := []string{field.Type}
		seed := spec.Initial
		if old, ok := checkpoint.ParseKeyset(prev[resource]); ok && len(old.Shards) > 0 {
			if old.Mode == checkpoint.ModeIncremental && slices.Equal(old.Cols, cols) && len(old.Shards) == 1 {
				old.Types = types
				plan[resource] = old.ToCheckpoint(resource)
				continue
			}
			// Migrate the former [pagination cursor, watermark] checkpoint shape.
			for i, col := range old.Cols {
				if col == checkpointKey && i < len(old.Shards[0].Key) {
					seed = old.Shards[0].Key[i]
					break
				}
			}
		}
		plan[resource] = checkpoint.KeysetCheckpoint{
			Mode: checkpoint.ModeIncremental, Cols: cols, Types: types,
			Shards: []checkpoint.KeysetShard{{Key: watermarkKey(seed)}},
		}.ToCheckpoint(resource)
	}
	s.incrementalLookbacks = lookbacks
	return plan, nil
}

func incrementalField(resource manifest.Resource) (manifest.FieldSpec, bool) {
	if resource.Incremental == nil {
		return manifest.FieldSpec{}, false
	}
	for _, field := range resource.Fields {
		if field.Name == resource.Incremental.CursorField {
			return field, true
		}
	}
	return manifest.FieldSpec{}, false
}

func incrementalCheckpointKey(spec manifest.IncrementalSpec) string {
	if spec.CheckpointKey != "" {
		return spec.CheckpointKey
	}
	return spec.CursorField
}

func watermarkKey(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

func incrementalCursorWarning(resource manifest.Resource) string {
	var warnings []string
	if resource.Incremental.Comparator == "lex" || resource.Incremental.Comparator == "" {
		warnings = append(warnings, "Lexical cursors must have a representation whose byte ordering matches source ordering")
	}
	if resource.Parent != nil {
		warnings = append(warnings, "Fan-out resources use one aggregate watermark; configure a lookback large enough to cover late arrivals during extraction")
	}
	return strings.Join(warnings, "; ")
}

// incrementalRecordReducer makes record keys monotonic even when a manifest
// resource internally fans out concurrent HTTP requests. The outer pipeline is
// ordered, so persisting the last emitted key then persists the maximum value
// observed before it.
type incrementalRecordReducer struct {
	source *Source
	seeds  map[string]map[string]string
	marks  map[string]*atomicwatermark.Watermark
}

func newIncrementalRecordReducer(source *Source, seeds map[string]map[string]string) *incrementalRecordReducer {
	return &incrementalRecordReducer{source: source, seeds: seeds, marks: map[string]*atomicwatermark.Watermark{}}
}

func (r *incrementalRecordReducer) record(rec pipeline.Record) (filament.Record, error) {
	spec, ok := r.source.incrementalResources[rec.Resource]
	if !ok {
		base := r.source.baseResourceName(rec.Resource)
		spec, ok = r.source.incrementalResources[base]
	}
	if !ok {
		return toIngestionRecord(rec), nil
	}
	checkpointKey := incrementalCheckpointKey(spec)
	mark := r.marks[rec.Resource]
	if mark == nil {
		cmp, err := atomicwatermark.ForName(spec.Comparator)
		if err != nil {
			return filament.Record{}, err
		}
		seed := r.seeds[rec.Resource][checkpointKey]
		if seed == "" {
			seed = r.seeds[r.source.baseResourceName(rec.Resource)][checkpointKey]
		}
		mark, err = atomicwatermark.New(cmp, seed)
		if err != nil {
			return filament.Record{}, err
		}
		r.marks[rec.Resource] = mark
	}
	if value := rec.Watermarks[checkpointKey]; value != "" {
		if _, err := mark.Observe(value); err != nil {
			return filament.Record{}, fmt.Errorf("incremental %q watermark: %w", rec.Resource, err)
		}
	}
	out := toIngestionRecord(rec)
	out.Key = watermarkKey(mark.Current())
	return out, nil
}
