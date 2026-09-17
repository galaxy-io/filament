package hubspot

import (
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
)

var initialWatermark = time.Unix(0, 0).UTC()

const defaultLookback = 5 * time.Minute

// CursorColumns exposes only the modification property supported by the API.
func (s *Source) CursorColumns(_ context.Context, name string) ([]filament.CursorColumn, error) {
	r, err := lookupResource(name)
	if err != nil {
		return nil, err
	}
	if r.modified == "" {
		return nil, nil
	}
	fields := schemaFor(r).Fields
	return []filament.CursorColumn{{
		SchemaField: fields[len(fields)-1], Eligible: true, Recommended: true, Rank: 1, SupportsLookback: true,
		Warning: "Interrupted reads replay the previous interval; use upsert to reconcile overlapping records",
	}}, nil
}

// PlanResume deliberately restarts full reads; a listing token is not a durable watermark.
func (s *Source) PlanResume(_ context.Context, names []string, _ map[string]filament.Checkpoint) (map[string]filament.Checkpoint, error) {
	for _, name := range names {
		if _, err := lookupResource(name); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// PlanIncremental preserves completed watermarks and seeds initial GET backfills.
func (s *Source) PlanIncremental(_ context.Context, names []string, previous map[string]filament.Checkpoint, cursors map[string]filament.ResourceCursorConfig) (map[string]filament.Checkpoint, error) {
	plans := make(map[string]filament.Checkpoint, len(names))
	if s.lookbacks == nil {
		s.lookbacks = make(map[string]time.Duration)
	}
	for _, name := range names {
		r, err := lookupResource(name)
		if err != nil {
			return nil, err
		}
		if r.modified == "" {
			return nil, fmt.Errorf("hubspot source: %s supports full reads only", name)
		}
		lookback := defaultLookback
		if config, ok := cursors[name]; ok {
			if config.Field != "" && config.Field != "modified_at" {
				return nil, fmt.Errorf("hubspot source: %s cursor must be modified_at", name)
			}
			if config.LookbackSeconds < 0 || config.LookbackSeconds > math.MaxInt64/int64(time.Second) {
				return nil, fmt.Errorf("hubspot source: %s lookback is out of range", name)
			}
			lookback = time.Duration(config.LookbackSeconds) * time.Second
		}
		s.lookbacks[name] = lookback
		mark := initialWatermark
		if previous[name] != nil {
			mark, err = planWatermark(name, previous[name])
			if err != nil {
				return nil, err
			}
		}
		plans[name] = watermarkPlan(name, mark)
	}
	return plans, nil
}

func watermarkPlan(name string, mark time.Time) *filament.CheckpointData {
	return (checkpoint.KeysetCheckpoint{
		Mode: checkpoint.ModeIncremental, Cols: []string{"modified_at"}, Types: []string{"timestamptz"},
		Shards: []checkpoint.KeysetShard{{Key: []string{mark.UTC().Format(time.RFC3339Nano)}}},
	}).ToCheckpoint(name)
}

func planWatermark(name string, cp filament.Checkpoint) (time.Time, error) {
	plan, ok := checkpoint.ParseKeyset(cp)
	if !ok || cp.Resource() != name || plan.Mode != checkpoint.ModeIncremental ||
		!slices.Equal(plan.Cols, []string{"modified_at"}) || !slices.Equal(plan.Types, []string{"timestamptz"}) ||
		len(plan.Shards) != 1 || len(plan.Shards[0].Key) != 1 {
		return time.Time{}, fmt.Errorf("hubspot source: incompatible checkpoint for %s; reset the resource checkpoint", name)
	}
	mark, err := time.Parse(time.RFC3339Nano, plan.Shards[0].Key[0])
	if err != nil || mark.Before(initialWatermark) {
		return time.Time{}, fmt.Errorf("hubspot source: invalid checkpoint timestamp for %s", name)
	}
	return mark, nil
}
