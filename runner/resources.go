package runner

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
)

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
