// Package appstoreconnect reads Apple App Store Connect data through Filament's
// native source contracts using ES256 JWT authentication, JSON:API pagination,
// and streaming gzipped TSV decompression for Analytics and Sales reports.
package appstoreconnect

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

// Source owns one run's HTTP client, configuration, and cursor settings.
type Source struct {
	client       *client
	vendorNumber string
	appIDs       []string
	startDate    time.Time
	lookbacks    map[string]time.Duration
	now          func() time.Time
}

// New returns an unconfigured source.
func New() *Source { return &Source{now: time.Now} }

var (
	_ filament.Source               = (*Source)(nil)
	_ filament.ConfigValidatable    = (*Source)(nil)
	_ filament.LiveValidatable      = (*Source)(nil)
	_ filament.Discoverable         = (*Source)(nil)
	_ filament.ResourcePlanner      = (*Source)(nil)
	_ filament.SchemaProvider       = (*Source)(nil)
	_ filament.CursorColumnProvider = (*Source)(nil)
	_ filament.IncrementalPlanner   = (*Source)(nil)
	_ filament.ResumePlanner        = (*Source)(nil)
	_ filament.Resumable            = (*Source)(nil)
)

// Validate checks configuration and parses the ECDSA P-256 private key without network requests.
func (s *Source) Validate(cfg filament.Config) error {
	issuerID := strings.TrimSpace(cfg.String("issuer_id"))
	if issuerID == "" {
		return fmt.Errorf("appstoreconnect source: issuer_id is required")
	}
	keyID := strings.TrimSpace(cfg.String("key_id"))
	if keyID == "" {
		return fmt.Errorf("appstoreconnect source: key_id is required")
	}
	rawKey := strings.TrimSpace(cfg.Secret("private_key"))
	if rawKey == "" {
		return fmt.Errorf("appstoreconnect source: private_key is required")
	}
	if _, err := parseECPrivateKey(rawKey); err != nil {
		return fmt.Errorf("appstoreconnect source: invalid private_key: %w", err)
	}
	if rawStart := strings.TrimSpace(cfg.String("start_date")); rawStart != "" {
		if _, err := time.Parse("2006-01-02", rawStart); err != nil {
			return fmt.Errorf("appstoreconnect source: invalid start_date %q (expected YYYY-MM-DD)", rawStart)
		}
	}
	if rps := cfg.Int("requests_per_second"); rps < 0 {
		return fmt.Errorf("appstoreconnect source: requests_per_second must be non-negative")
	}
	return nil
}

// Configure initializes the authenticated client for a run.
func (s *Source) Configure(_ context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	c, err := newClient(
		strings.TrimSpace(cfg.String("issuer_id")),
		strings.TrimSpace(cfg.String("key_id")),
		strings.TrimSpace(cfg.Secret("private_key")),
	)
	if err != nil {
		return err
	}
	if rps := cfg.Int("requests_per_second"); rps > 0 {
		c.limiter = rate.NewLimiter(rate.Limit(rps), 1)
	}
	s.client = c
	s.vendorNumber = strings.TrimSpace(cfg.String("vendor_number"))
	s.appIDs = parseCSVList(cfg.String("app_ids"))
	s.startDate = time.Time{}
	if rawStart := strings.TrimSpace(cfg.String("start_date")); rawStart != "" {
		parsed, _ := time.Parse("2006-01-02", rawStart)
		s.startDate = parsed.UTC()
	}
	s.lookbacks = make(map[string]time.Duration)
	return nil
}

func parseCSVList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// TestConnection verifies credentials with a lightweight read against /v1/apps.
func (s *Source) TestConnection(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	c, err := newClient(
		strings.TrimSpace(cfg.String("issuer_id")),
		strings.TrimSpace(cfg.String("key_id")),
		strings.TrimSpace(cfg.Secret("private_key")),
	)
	if err != nil {
		return err
	}
	defer c.http.CloseIdleConnections()
	var page jsonAPIPage
	if err := c.requestJSON(ctx, "apps", http.MethodGet, "/v1/apps?limit=1", &page); err != nil {
		return fmt.Errorf("appstoreconnect source: list apps: %w", err)
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
		return fmt.Errorf("appstoreconnect source: extract before configure")
	}
	s.client.observe = opts.Observe
	upper := s.now().UTC()
	selected, err := s.PlanResources(ctx, opts.Resources, opts.Selectors)
	if err != nil {
		return err
	}
	jobs := make([]func(context.Context) error, 0, len(selected))
	for _, name := range selected {
		r, err := lookupResource(name)
		if err != nil {
			return err
		}
		if r.name == "sales_reports" && s.vendorNumber == "" {
			return fmt.Errorf("appstoreconnect source: vendor_number is required to extract sales_reports")
		}
		if plans[name] != nil {
			if r.cursorCol == "" {
				return fmt.Errorf("appstoreconnect source: %s does not support incremental reads", name)
			}
			if _, err := planWatermark(name, plans[name]); err != nil {
				return err
			}
		}
		jobs = append(jobs, func(ctx context.Context) error {
			if err := s.readResource(ctx, sink, r, opts.Limit, upper, plans[r.name]); err != nil {
				return fmt.Errorf("appstoreconnect source: %s: %w", r.name, err)
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
