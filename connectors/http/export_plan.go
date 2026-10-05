package httpapi

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/internal/export"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func (s *Source) exportPlanState(res manifest.Resource, cp filament.Checkpoint, inc bool) (export.RunState, error) {
	return s.connector.exportRuntime(res).PlanState(res, cp, inc)
}

func (s *Source) planExportIncremental(res manifest.Resource, previous filament.Checkpoint) (filament.Checkpoint, error) {
	return s.connector.exportRuntime(res).PlanIncremental(res, previous)
}

// PlanIncrementalResume restores attempt-local jobs separately from durable watermarks.
func (s *Source) PlanIncrementalResume(_ context.Context, planned, attempt map[string]filament.Checkpoint) (map[string]filament.Checkpoint, error) {
	result := make(map[string]filament.Checkpoint, len(planned))
	for resource, cp := range planned {
		res, ok := s.manifestResource(resource)
		if !ok || !export.Scoped(res) {
			return nil, fmt.Errorf("unknown export resource %q", resource)
		}
		restored, err := s.connector.exportRuntime(res).Restore(res, cp, attempt[resource])
		if err != nil {
			return nil, err
		}
		result[resource] = restored
	}
	return result, nil
}
