package appstoreconnect

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
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
	var serverURL string
	var observedURIs []string
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
			_, _ = fmt.Fprintf(w, `{"data":[{"id":"seg-1","type":"analyticsReportSegments","attributes":{"url":"%s/download/seg-1.tsv.gz?X-Amz-Signature=secret123"}}]}`, serverURL)
		case req.URL.Path == "/download/seg-1.tsv.gz":
			w.Header().Set("Content-Type", "application/a-gzip")
			_, _ = w.Write(gzipBytes(t, "Date\tApp Name\tSessions\n2026-11-16\tGalaxy App\t42\n2026-11-16\tGalaxy App\t18\n"))
		case strings.HasPrefix(req.URL.Path, "/v1/salesReports"):
			reportDate := req.URL.Query().Get("filter[reportDate]")
			if reportDate == "2026-11-15" {
				w.Header().Set("Content-Type", "application/a-gzip")
				_, _ = w.Write(gzipBytes(t, "Provider\tSKU\tCountry Code\tProduct Type Identifier\tUnits\nAPPLE\tSKU-1\tUS\t1F\t99\n"))
				return
			}
			// Simulate 2026-11-16 report not yet published (404) and older days (e.g. 2026-10-20) having no sales (404 outside 7-day retry window).
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"id":"not-found","code":"NOT_FOUND"}]}`))
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
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

	// 1. Initial bootstrap on sales_reports: older 404s (2026-10-20..11-09) are outside the 7-day retry window,
	// 2026-11-10..11-14 are within the 7-day window (earliest missing in window is 2026-11-10).
	// Now set start_date to 2026-11-15 so 2026-11-15 succeeds and 2026-11-16 is the earliest missing day.
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

	// Resume from the checkpoint produced by the first run.
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
	if gotKey := salesIncSink.rows[len(salesIncSink.rows)-1].key; !slices.Equal(gotKey, []string{"2026-11-16T00:00:00Z"}) {
		t.Fatalf("incremental sales watermark = %v, want [2026-11-16T00:00:00Z]", gotKey)
	}

	// 2. Verify customer_reviews and analytics_reports + URI query redaction.
	prev := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	for _, resName := range []string{"customer_reviews", "analytics_reports"} {
		plans, err := s.PlanIncremental(t.Context(), []string{resName}, map[string]filament.Checkpoint{resName: watermarkPlan(resName, prev)}, nil)
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
	}
	for _, u := range observedURIs {
		if strings.Contains(u, "X-Amz-Signature") || strings.Contains(u, "?") {
			t.Fatalf("unredacted query in progress URI: %q", u)
		}
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
