package source

import (
	"context"
	"slices"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/galaxy-io/filament"
)

// Discover lists stored subject patterns as selectable logical resources.
func (s *Source) Discover(ctx context.Context, _ filament.DiscoverOpts) (filament.DiscoverResult, error) {
	js, err := jetstream.New(s.conn)
	if err != nil {
		return filament.DiscoverResult{}, err
	}
	names := js.StreamNames(ctx)
	var subjects []string
	for name := range names.Name() {
		info, err := s.js.StreamInfo(name, nats.Context(ctx))
		if err != nil {
			return filament.DiscoverResult{}, err
		}
		subjects = append(subjects, info.Config.Subjects...)
	}
	if err := names.Err(); err != nil {
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
