package source

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
)

// Discover returns configured RabbitMQ Stream resources after verifying they exist.
func (s *Source) Discover(ctx context.Context, _ filament.DiscoverOpts) (filament.DiscoverResult, error) {
	if s.env == nil {
		return filament.DiscoverResult{}, fmt.Errorf("rabbitmq: discover before configure")
	}
	streams, err := selectedStreams(nil, s.defaultStreams)
	if err != nil {
		return filament.DiscoverResult{}, err
	}
	resources := make([]filament.Resource, 0, len(streams))
	for _, name := range streams {
		if err := ctx.Err(); err != nil {
			return filament.DiscoverResult{}, err
		}
		exists, err := s.env.StreamExists(name)
		if err != nil {
			return filament.DiscoverResult{}, fmt.Errorf("rabbitmq: check stream %q: %w", name, err)
		}
		if !exists {
			return filament.DiscoverResult{}, fmt.Errorf("rabbitmq: configured stream %q does not exist", name)
		}
		resources = append(resources, filament.Resource{Name: name, Selectable: true})
	}
	return filament.DiscoverResult{Resources: resources}, nil
}
