package source

import (
	"context"
	"slices"

	"github.com/galaxy-io/filament"
)

// Discover lists stored subject patterns as selectable logical resources.
func (s *Source) Discover(ctx context.Context, _ filament.DiscoverOpts) (filament.DiscoverResult, error) {
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
