// Package hubspot reads HubSpot CRM data through Filament's native source contracts.
// Full reads list object IDs and hydrate their properties; incremental reads use
// bounded search intervals and advance a watermark only after completing a resource.
package hubspot

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

// Source owns one run's HTTP client, catalog, and cursor configuration.
type Source struct {
	client     *client
	properties map[string][]string
	lookbacks  map[string]time.Duration
	now        func() time.Time
}

// New returns an unconfigured source.
func New() *Source { return &Source{now: time.Now} }

var (
	_ filament.Source               = (*Source)(nil)
	_ filament.LiveValidatable      = (*Source)(nil)
	_ filament.Discoverable         = (*Source)(nil)
	_ filament.ResourcePlanner      = (*Source)(nil)
	_ filament.SchemaProvider       = (*Source)(nil)
	_ filament.CursorColumnProvider = (*Source)(nil)
	_ filament.IncrementalPlanner   = (*Source)(nil)
	_ filament.ResumePlanner        = (*Source)(nil)
	_ filament.Resumable            = (*Source)(nil)
)

// Validate checks configuration without making network requests.
func (s *Source) Validate(cfg filament.Config) error {
	if strings.TrimSpace(cfg.Secret("api_key")) == "" {
		return fmt.Errorf("hubspot source: api_key is required")
	}
	return nil
}

// Configure creates the client; catalog reads are deferred until needed.
func (s *Source) Configure(_ context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	s.client = newClient(cfg.Secret("api_key"))
	s.properties = make(map[string][]string)
	s.lookbacks = make(map[string]time.Duration)
	return nil
}

// TestConnection validates the token with an account-level read.
func (s *Source) TestConnection(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	c := newClient(cfg.Secret("api_key"))
	defer c.http.CloseIdleConnections()
	var account struct {
		PortalID int64 `json:"portalId"`
	}
	if err := c.request(ctx, "", http.MethodGet, "/account-info/"+apiVersion+"/details", nil, &account); err != nil {
		return fmt.Errorf("hubspot source: account details: %w", err)
	}
	return nil
}

// Teardown releases idle HTTP connections.
func (s *Source) Teardown(context.Context) error {
	if s.client != nil {
		s.client.http.CloseIdleConnections()
		s.client = nil
	}
	return nil
}

// Extract reads the selected resources in full.
func (s *Source) Extract(ctx context.Context, sink arrowbatch.Inlet, opts filament.ExtractOpts) error {
	return s.extract(ctx, sink, opts, nil)
}

// ExtractFrom reads incremental plans; nil entries are full reads.
func (s *Source) ExtractFrom(ctx context.Context, sink arrowbatch.Inlet, opts filament.ExtractOpts, plans map[string]filament.Checkpoint) error {
	return s.extract(ctx, sink, opts, plans)
}

func (s *Source) extract(ctx context.Context, sink arrowbatch.Inlet, opts filament.ExtractOpts, plans map[string]filament.Checkpoint) error {
	if s.client == nil {
		return fmt.Errorf("hubspot source: extract before configure")
	}
	s.client.observe = opts.Observe
	upper := s.now().UTC()
	selected, err := s.PlanResources(ctx, opts.Resources, nil)
	if err != nil {
		return err
	}
	jobs := make([]func(context.Context) error, 0, len(selected))
	for _, name := range selected {
		r, err := lookupResource(name)
		if err != nil {
			return err
		}
		// Prepare shared metadata before launching readers; no concurrent cache writes.
		if r.objectType != "" {
			if err := s.loadProperties(ctx, r); err != nil {
				return err
			}
		}
		if plans[name] != nil {
			if r.modified == "" {
				return fmt.Errorf("hubspot source: %s does not support incremental reads", name)
			}
			if _, err := planWatermark(name, plans[name]); err != nil {
				return err
			}
		}
		jobs = append(jobs, func(ctx context.Context) error {
			if err := s.readResource(ctx, sink, r, opts.Limit, upper, plans[r.name]); err != nil {
				return fmt.Errorf("hubspot source: %s: %w", r.name, err)
			}
			return nil
		})
	}
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(max(opts.Parallelism, 1))
	for _, job := range jobs {
		group.Go(func() error { return job(ctx) })
	}
	return group.Wait()
}
