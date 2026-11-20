package appstoreconnect

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/pipeline"
)

var testTime = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

func gzipBytes(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type capturedRow struct {
	id  string
	key []string
}

type captureSink struct {
	mu   sync.Mutex
	rows []capturedRow
}

func (*captureSink) Spec() filament.SinkSpec {
	return filament.SinkSpec{Name: "capture", Capabilities: filament.SinkCapabilities{
		WritePolicies: filament.WriteCapabilities(filament.IngestionIncrementalUpsert, filament.IngestionFullUpsert, filament.IngestionFullReplace),
	}}
}
func (*captureSink) Name() string                                 { return "capture" }
func (*captureSink) Open(context.Context, filament.RunSpec) error { return nil }
func (*captureSink) Begin(context.Context) error                  { return nil }
func (*captureSink) Commit(context.Context) error                 { return nil }
func (*captureSink) Abort(context.Context) error                  { return nil }
func (*captureSink) Close(context.Context) error                  { return nil }
func (s *captureSink) Apply(_ context.Context, b *arrowbatch.Batch, _ filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := 0; i < b.NumRows(); i++ {
		s.rows = append(s.rows, capturedRow{
			id:  fmt.Sprint(b.Rows().Column(0).GetOneForMarshal(i)),
			key: slices.Clone(b.Last.Key),
		})
	}
	return filament.WriteReceipt{Rows: b.NumRows(), WriteCRC: b.IntegrityCRC()}, nil
}

func TestAnalyticsAndSalesGzippedTSVIncrementalExtraction(t *testing.T) {
	pemKey, _ := generateTestP8Key(t)
	analyticsFixture := gzipBytes(t, "Date\tApp Name\tTerritory\tSessions\n2026-11-15\tGalaxy App\tUS\t42\n2026-11-15\tGalaxy App\tGB\t18\n")
	salesFixture := gzipBytes(t, "Provider\tSKU\tCountry Code\tProduct Type Identifier\tUnits\nAPPLE\tSKU-1\tUS\t1F\t99\n")

	var serverURL string
	var observedURIs []string
	var segDownloadAttempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.URL.Path == "/v1/apps":
			_, _ = w.Write([]byte(`{"data":[{"id":"app-1","type":"apps","attributes":{"bundleId":"io.galaxy.app","name":"Galaxy App"}}]}`))
		case req.URL.Path == "/v1/apps/app-1/customerReviews":
			_, _ = w.Write([]byte(`{"data":[{"id":"rev-1","type":"customerReviews","attributes":{"rating":5,"title":"Great","createdDate":"2026-11-16T10:00:00Z"}}]}`))
		case req.URL.Path == "/v1/apps/app-1/analyticsReportRequests":
			_, _ = w.Write([]byte(`{"data":[{"id":"req-1","type":"analyticsReportRequests","attributes":{"accessType":"ONGOING"}}]}`))
		case req.URL.Path == "/v1/analyticsReportRequests/req-1/reports":
			_, _ = w.Write([]byte(`{"data":[{"id":"rep-1","type":"analyticsReports","attributes":{"category":"APP_USAGE","name":"App Sessions"}}]}`))
		case req.URL.Path == "/v1/analyticsReports/rep-1/instances":
			if req.URL.Query().Get("filter[granularity]") != "DAILY" {
				t.Errorf("expected filter[granularity]=DAILY, got %q", req.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"inst-1","type":"analyticsReportInstances","attributes":{"processingDate":"2026-11-16"}}]}`))
		case req.URL.Path == "/v1/analyticsReportInstances/inst-1/segments":
			_, _ = fmt.Fprintf(w, `{"data":[{"id":"seg-1","type":"analyticsReportSegments","attributes":{"url":"%s/download/seg-1.tsv.gz?X-Amz-Signature=expired123"}}]}`, serverURL)
		case req.URL.Path == "/v1/analyticsReportSegments/seg-1":
			_, _ = fmt.Fprintf(w, `{"data":{"id":"seg-1","type":"analyticsReportSegments","attributes":{"url":"%s/download/seg-1.tsv.gz?X-Amz-Signature=fresh456"}}}`, serverURL)
		case req.URL.Path == "/download/seg-1.tsv.gz":
			if segDownloadAttempts.Add(1) == 1 && req.URL.Query().Get("X-Amz-Signature") == "expired123" {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"errors":[{"id":"expired","code":"FORBIDDEN"}]}`))
				return
			}
			w.Header().Set("Content-Type", "application/a-gzip")
			_, _ = w.Write(analyticsFixture)
		case strings.HasPrefix(req.URL.Path, "/v1/salesReports"):
			reportDate := req.URL.Query().Get("filter[reportDate]")
			if reportDate == "2026-11-15" {
				w.Header().Set("Content-Type", "application/a-gzip")
				_, _ = w.Write(salesFixture)
				return
			}
			// Simulate 2026-11-16 report not yet published (404) and older days (e.g. 2026-10-20) having no sales (404 outside 7-day retry window).
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"id":"not-found","code":"NOT_FOUND"}]}`))
		default:
			t.Errorf("unexpected path: %s", req.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	s := New()
	cfg := filament.NewConfig(map[string]any{
		"issuer_id":           "issuer-1",
		"key_id":              "KEY1234567",
		"private_key":         pemKey,
		"vendor_number":       "88001122",
		"app_ids":             "app-1",
		"start_date":          "2026-10-20",
		"requests_per_second": 10,
	})
	if err := s.Configure(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Teardown(t.Context()) }()
	if s.client.limiter.Limit() != rate.Limit(10) {
		t.Fatalf("limiter = %v, want 10", s.client.limiter.Limit())
	}
	s.client.baseURL, s.client.limiter = server.URL, rate.NewLimiter(rate.Inf, 1)
	s.now = func() time.Time { return testTime }

	// 1. Initial bootstrap on sales_reports: 2026-11-15 succeeds and 2026-11-16 is the earliest missing day in the 7-day retry window.
	s.startDate = time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	plans, err := s.PlanIncremental(t.Context(), []string{"sales_reports"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	salesSink := &captureSink{}
	plan, err := filament.ResolveIngestionPlan(t.Context(), s, salesSink, filament.RunSpec{
		Resources:      []string{"sales_reports"},
		IngestionTypes: map[string]filament.IngestionType{"sales_reports": filament.IngestionIncrementalUpsert},
	})
	if err != nil {
		t.Fatal(err)
	}
	p := pipeline.New(pipeline.Config{Sink: salesSink, WritePolicies: plan.WritePolicies, Options: filament.RunOptions{BatchMaxRows: 1}})
	p.Start(t.Context())
	err = s.ExtractFrom(t.Context(), p.Records(), filament.ExtractOpts{
		Resources: []string{"sales_reports"},
		Observe: func(prog filament.SourceProgress) {
			if prog.URI != "" {
				observedURIs = append(observedURIs, prog.URI)
			}
		},
	}, plans)
	p.CloseIngest(err)
	if waitErr := p.Wait(); waitErr != nil {
		t.Fatal(waitErr)
	}
	if len(salesSink.rows) != 1 {
		t.Fatalf("sales_reports rows = %d, want 1", len(salesSink.rows))
	}
	if gotKey := salesSink.rows[0].key; !slices.Equal(gotKey, []string{"2026-11-16T00:00:00Z"}) {
		t.Fatalf("initial sales watermark = %v, want [2026-11-16T00:00:00Z]", gotKey)
	}
	firstSavedMark, err := time.Parse(time.RFC3339Nano, salesSink.rows[0].key[0])
	if err != nil {
		t.Fatal(err)
	}
	firstCheckpoint := watermarkPlan("sales_reports", firstSavedMark)

	// Resume on 2026-11-30 when 2026-11-15 is the last day with sales and 2026-11-16..22 are outside the 7-day retry window.
	// The checkpoint must advance past 2026-11-16 to the earliest missing day still in the 7-day retry window (2026-11-23).
	s.now = func() time.Time { return time.Date(2026, 11, 30, 12, 0, 0, 0, time.UTC) }
	incPlans, err := s.PlanIncremental(t.Context(), []string{"sales_reports"}, map[string]filament.Checkpoint{"sales_reports": firstCheckpoint}, nil)
	if err != nil {
		t.Fatal(err)
	}
	salesIncSink := &captureSink{}
	pInc := pipeline.New(pipeline.Config{Sink: salesIncSink, WritePolicies: plan.WritePolicies, Options: filament.RunOptions{BatchMaxRows: 1}})
	pInc.Start(t.Context())
	err = s.ExtractFrom(t.Context(), pInc.Records(), filament.ExtractOpts{Resources: []string{"sales_reports"}}, incPlans)
	pInc.CloseIngest(err)
	if waitErr := pInc.Wait(); waitErr != nil {
		t.Fatal(waitErr)
	}
	if gotKey := salesIncSink.rows[len(salesIncSink.rows)-1].key; !slices.Equal(gotKey, []string{"2026-11-23T00:00:00Z"}) {
		t.Fatalf("incremental sales watermark after quiet days = %v, want [2026-11-23T00:00:00Z]", gotKey)
	}

	// Resume again from 2026-11-23 on 2026-12-02 when no rows at all are emitted in the scanned window [2026-11-22..2026-12-01]:
	// w.Drain must still emit a completion checkpoint advancing to 2026-11-25 (earliest missing day in [2026-11-25..12-01]).
	s.now = func() time.Time { return time.Date(2026, 12, 2, 12, 0, 0, 0, time.UTC) }
	secondMark, err := time.Parse(time.RFC3339Nano, "2026-11-23T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	secondPlans, err := s.PlanIncremental(t.Context(), []string{"sales_reports"}, map[string]filament.Checkpoint{"sales_reports": watermarkPlan("sales_reports", secondMark)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var drainedCheckpoints []*filament.CheckpointData
	pEmpty := pipeline.New(pipeline.Config{
		Sink:          &captureSink{},
		WritePolicies: plan.WritePolicies,
		Options:       filament.RunOptions{BatchMaxRows: 1},
		Emit: func(f events.Fact) {
			if ev, ok := f.Detail.(events.BatchWrittenEvent); ok && ev.Checkpoint != nil {
				drainedCheckpoints = append(drainedCheckpoints, ev.Checkpoint)
			}
		},
	})
	pEmpty.Start(t.Context())
	err = s.ExtractFrom(t.Context(), pEmpty.Records(), filament.ExtractOpts{Resources: []string{"sales_reports"}}, secondPlans)
	pEmpty.CloseIngest(err)
	if waitErr := pEmpty.Wait(); waitErr != nil {
		t.Fatal(waitErr)
	}
	if len(drainedCheckpoints) == 0 {
		t.Fatal("expected rowless sales scan to emit a drained checkpoint delta")
	}
	s.now = func() time.Time { return testTime }

	// 2. Verify customer_reviews and analytics_reports (including 403 expired segment URL refresh, event-date row ID, Billing Period dimension, and URI query redaction).
	prev := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	for _, resName := range []string{"customer_reviews", "analytics_reports"} {
		plans, err := s.PlanIncremental(t.Context(), []string{resName}, map[string]filament.Checkpoint{resName: watermarkPlanWithApps(resName, prev, "app-1")}, nil)
		if err != nil {
			t.Fatal(err)
		}
		sink := &captureSink{}
		plan, err := filament.ResolveIngestionPlan(t.Context(), s, sink, filament.RunSpec{
			Resources:      []string{resName},
			IngestionTypes: map[string]filament.IngestionType{resName: filament.IngestionIncrementalUpsert},
		})
		if err != nil {
			t.Fatal(err)
		}
		p := pipeline.New(pipeline.Config{Sink: sink, WritePolicies: plan.WritePolicies, Options: filament.RunOptions{BatchMaxRows: 1}})
		p.Start(t.Context())
		err = s.ExtractFrom(t.Context(), p.Records(), filament.ExtractOpts{
			Resources: []string{resName},
			Observe: func(prog filament.SourceProgress) {
				if prog.URI != "" {
					observedURIs = append(observedURIs, prog.URI)
				}
			},
		}, plans)
		p.CloseIngest(err)
		if waitErr := p.Wait(); waitErr != nil {
			t.Fatal(waitErr)
		}
		if len(sink.rows) == 0 {
			t.Fatalf("%s emitted 0 rows", resName)
		}
		if resName == "analytics_reports" {
			if segDownloadAttempts.Load() < 2 {
				t.Fatalf("expected segment download to retry after 403 URL refresh, got %d attempts", segDownloadAttempts.Load())
			}
			if len(sink.rows) != 2 {
				t.Fatalf("analytics_reports rows = %d, want 2", len(sink.rows))
			}
			if sink.rows[0].id == sink.rows[1].id {
				t.Fatalf("expected distinct analytics_reports row IDs, got identical %q", sink.rows[0].id)
			}
			// Even though processingDate is 2026-11-16, the row's own Date dimension is 2026-11-15, so the ID is keyed on 2026-11-15.
			for _, r := range sink.rows {
				if !strings.HasPrefix(r.id, "app-1:App Sessions:2026-11-15:") {
					t.Fatalf("unexpected analytics_reports composite id format (expected event Date 2026-11-15): %q", r.id)
				}
			}
			// Verify that restating a measure (Sessions 42 -> 99) for the same dimensions preserves the row ID,
			// while changing numeric dimensions (Hour "1" -> "2" or Billing Period "1" -> "2") produces distinct dimension hashes.
			h1 := analyticsDimensionHash("APP_USAGE", "App Sessions", map[string]string{"Date": "2026-11-15", "App Name": "Galaxy App", "Territory": "US", "Hour": "1", "Sessions": "42"})
			h2 := analyticsDimensionHash("APP_USAGE", "App Sessions", map[string]string{"Date": "2026-11-15", "App Name": "Galaxy App", "Territory": "US", "Hour": "1", "Sessions": "99"})
			if h1 != h2 {
				t.Fatalf("expected restated measure to preserve dimension hash, got %q != %q", h1, h2)
			}
			h3 := analyticsDimensionHash("APP_USAGE", "App Sessions", map[string]string{"Date": "2026-11-15", "App Name": "Galaxy App", "Territory": "US", "Hour": "2", "Sessions": "42"})
			if h1 == h3 {
				t.Fatalf("expected distinct dimension hash for different numeric Hour dimension, got identical %q", h1)
			}
			b1 := analyticsDimensionHash("COMMERCE", "Subscription Event", map[string]string{"Event Date": "2026-11-15", "Subscription Name": "Pro", "Billing Period": "1", "Quantity": "5"})
			b2 := analyticsDimensionHash("COMMERCE", "Subscription Event", map[string]string{"Event Date": "2026-11-15", "Subscription Name": "Pro", "Billing Period": "2", "Quantity": "5"})
			if b1 == b2 {
				t.Fatalf("expected distinct dimension hash for different Billing Period values, got identical %q", b1)
			}
		}
	}
	expectedSegmentURI := serverURL + "/download/seg-1.tsv.gz"
	if !slices.Contains(observedURIs, expectedSegmentURI) {
		t.Fatalf("expected observedURIs %v to contain redacted segment URI %q", observedURIs, expectedSegmentURI)
	}
	for _, u := range observedURIs {
		if strings.Contains(u, "X-Amz-Signature") || strings.Contains(u, "?") {
			t.Fatalf("unredacted query in progress URI: %q", u)
		}
	}

	// 3. Verify redactURLError strips signed query parameters from *url.Error.
	rawErr := &url.Error{Op: "Get", URL: "https:// downloads.apple.com/seg-1.tsv.gz?X-Amz-Signature=secret999", Err: errors.New("i/o timeout")}
	redactedErr := redactURLError(rawErr)
	if strings.Contains(redactedErr.Error(), "secret999") || strings.Contains(redactedErr.Error(), "X-Amz-Signature") {
		t.Fatalf("expected redactURLError to strip query string, got %q", redactedErr.Error())
	}
}

func TestSameOriginPreventsTokenLeakage(t *testing.T) {
	if !sameOrigin("https://api.appstoreconnect.apple.com/v1/apps", "https://api.appstoreconnect.apple.com") {
		t.Fatal("expected sameOrigin to match identical host")
	}
	if sameOrigin("https://api.appstoreconnect.apple.com.attacker.example/report.gz", "https://api.appstoreconnect.apple.com") {
		t.Fatal("sameOrigin matched attacker subdomain prefix")
	}
}
