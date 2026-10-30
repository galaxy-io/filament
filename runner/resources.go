package runner

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
)

// SelectColumns hands a run's per-resource column selection to the source.
// A selection on a source that cannot honor it fails the run rather than
// silently reading every column.
func SelectColumns(src filament.Source, columns map[string][]string) error {
	if len(columns) == 0 {
		return nil
	}
	selector, ok := src.(filament.ColumnSelector)
	if !ok {
		return fmt.Errorf("source %q does not support column selection", src.Spec().Name)
	}
	return selector.SelectColumns(columns)
}

// PlanResources resolves a run's requested resources to concrete source
// resources. An empty request means "all" for discoverable sources. Sources
// with private selector semantics can override this through ResourcePlanner.
func PlanResources(ctx context.Context, src filament.Source, resources, selectors []string) ([]string, error) {
	if planner, ok := src.(filament.ResourcePlanner); ok {
		planned, err := planner.PlanResources(ctx, resources, selectors)
		if err != nil {
			return nil, err
		}
		if len(planned) == 0 {
			return nil, fmt.Errorf("source %q planned no resources", src.Spec().Name)
		}
		return planned, nil
	}
	if len(resources) > 0 {
		return resources, nil
	}
	discoverable, ok := src.(filament.Discoverable)
	if !ok {
		return resources, nil
	}
	discovered, err := discoverable.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		return nil, fmt.Errorf("discover all resources: %w", err)
	}
	planned := make([]string, 0, len(discovered.Resources))
	for _, resource := range discovered.Resources {
		if resource.Selectable {
			planned = append(planned, resource.Name)
		}
	}
	if len(planned) == 0 {
		return nil, fmt.Errorf("source %q discovered no selectable resources", src.Spec().Name)
	}
	return planned, nil
}
