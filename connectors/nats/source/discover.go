package source

import (
	"context"
	"errors"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

// TestConnection verifies JetStream access using a temporary connection.
func (s *Source) TestConnection(ctx context.Context, cfg filament.Config) (err error) {
	temp := New()
	if err := temp.Configure(ctx, cfg); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, temp.Teardown(ctx)) }()
	_, err = temp.js.AccountInfo(nats.Context(ctx))
	return err
}

// Discover lists existing JetStream streams as selectable resources.
func (s *Source) Discover(ctx context.Context, _ filament.DiscoverOpts) (filament.DiscoverResult, error) {
	js, err := jetstream.New(s.conn)
	if err != nil {
		return filament.DiscoverResult{}, err
	}
	names := js.StreamNames(ctx)
	var resources []filament.Resource
	for name := range names.Name() {
		resources = append(resources, filament.Resource{Name: name, Selectable: true})
	}
	return filament.DiscoverResult{Resources: resources}, names.Err()
}

// Schema returns the fixed message envelope and subject column for a stream.
func (s *Source) Schema(_ context.Context, resource string) (rowmodel.Schema, error) {
	return Schema(resource)
}

var (
	_ filament.Source          = (*Source)(nil)
	_ filament.StreamSource    = (*Source)(nil)
	_ filament.SchemaProvider  = (*Source)(nil)
	_ filament.Discoverable    = (*Source)(nil)
	_ filament.LiveValidatable = (*Source)(nil)
)
