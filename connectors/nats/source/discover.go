package source

import (
	"context"
	"slices"

	"github.com/galaxy-io/filament"
)

// Discover lists configured stream names for explicit bindings, or stored
// subject patterns for managed consumers.
func (s *Source) Discover(ctx context.Context, _ filament.DiscoverOpts) (filament.DiscoverResult, error) {
	if len(s.bindings) > 0 {
		resources := make([]filament.Resource, 0, len(s.bindings))
		for _, binding := range s.bindings {
			resources = append(resources, filament.Resource{Name: binding.Stream, Selectable: true})
		}
		return filament.DiscoverResult{Resources: resources}, nil
	}
	streams := s.js.ListStreams(ctx)
	var subjects []string
	for info := range streams.Info() {
		subjects = append(subjects, info.Config.Subjects...)
	}
	if err := streams.Err(); err != nil {
		return filament.DiscoverResult{}, err
	}
	slices.Sort(subjects)
	subjects = slices.Compact(subjects)
	var resources []filament.Resource
	for _, subject := range subjects {
		resources = append(resources, filament.Resource{Name: subject, Selectable: true})
	}
	return filament.DiscoverResult{Resources: resources}, nil
}
