// Package sample implements the filament.Source interface as a synthetic generator
// (dev/test). It emits a configurable number of rows per requested resource with
// no external dependencies, serving as the zero-dep reference source — the
// counterpart to the stdout sink — for end-to-end wiring, demos, and tests.
package sample

import (
	"context"
	"fmt"
	"strconv"

	"github.com/galaxy-io/filament"
)

// defaultRows is emitted per resource when "rows" is not configured.
const defaultRows = 3

// Source generates synthetic records. One instance is created per run, then
// Configure sets the per-resource row count from the run's source config.
type Source struct {
	rows int
}

// New returns a generator with the default row count.
func New() *Source { return &Source{rows: defaultRows} }

var (
	_ filament.Source       = (*Source)(nil)
	_ filament.Discoverable = (*Source)(nil)
)

// Spec describes the generator's config fields, modes, and write policies.
func (s *Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name:           "sample",
		DisplayName:    "Sample Generator",
		Version:        "1",
		Modes:          []filament.ReadMode{filament.ModeFull},
		SourcePolicies: filament.SourcePolicies(filament.IngestionSnapshotReplace),
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "rows", Type: filament.FieldInt, Scope: filament.ScopePipeline, Help: "Rows to generate per resource"},
		}},
		Resources: filament.ResourceCapabilities{Discoverable: true},
	}
}

// Validate accepts any config; every field is optional.
func (s *Source) Validate(filament.Config) error { return nil }

// Configure reads the optional "rows" count from the source config.
func (s *Source) Configure(_ context.Context, cfg filament.Config) error {
	if cfg.Has("rows") {
		s.rows = cfg.Int("rows")
	}
	return nil
}

// Discover lists the synthetic users and orders resources with row estimates.
func (s *Source) Discover(context.Context, filament.DiscoverOpts) (filament.DiscoverResult, error) {
	rows := int64(s.rows)
	if rows <= 0 {
		rows = defaultRows
	}
	return filament.DiscoverResult{Resources: []filament.Resource{
		{Name: "users", Selectable: true, PrimaryKey: []string{"id"}, Estimated: rows},
		{Name: "orders", Selectable: true, PrimaryKey: []string{"id"}, Estimated: rows},
	}}, nil
}

// Extract emits rows synthetic records for each requested resource (defaulting to
// a single "items" resource when none are named), respecting cancellation and
// pipeline backpressure via the sink.
func (s *Source) Extract(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
	resources := opts.Resources
	if len(resources) == 0 {
		resources = []string{"items"}
	}
	rows := s.rows
	if rows <= 0 {
		rows = 1
	}
	for _, resource := range resources {
		for i := 0; i < rows; i++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			data := fmt.Appendf(nil, `{"resource":%q,"i":%d}`, resource, i)
			if err := sink.Push(filament.NewRecord(resource, strconv.Itoa(i), data)); err != nil {
				return err
			}
		}
	}
	return nil
}

// Teardown is a no-op; the generator holds no resources.
func (s *Source) Teardown(context.Context) error { return nil }
