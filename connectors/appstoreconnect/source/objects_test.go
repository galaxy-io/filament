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
			_, _ = w.Write([]byte(`{"data":[{"id":"inst-1","type":"analyticsReportInstances","attributes":{"processingDate":"2026-11-16"}}]}`))
		case req.URL.Path == "/v1/analyticsReportInstances/inst-1/segments":
			_, _ = fmt.Fprintf(w, `{"data":[{"id":"seg-1","type":"analyticsReportSegments","attributes":{"url":"%s/download/seg-1.tsv.gz"}}]}`, serverURL)
		case req.URL.Path == "/download/seg-1.tsv.gz":
			w.Header().Set("Content-Type", "application/a-gzip")
			_, _ = w.Write(gzipBytes(t, "Date\tApp Name\tSessions\n2026-11-16\tGalaxy App\t42\n2026-11-16\tGalaxy App\t18\n"))
		case strings.HasPrefix(req.URL.Path, "/v1/salesReports"):
			if req.URL.Query().Get("filter[reportDate]") == "2026-11-16" {
				w.Header().Set("Content-Type", "application/a-gzip")
				_, _ = w.Write(gzipBytes(t, "Provider\tSKU\tCountry Code\tProduct Type Identifier\tUnits\nAPPLE\tSKU-1\tUS\t1F\t99\n"))
				return
			}
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
		"issuer_id":     "issuer-1",
		"key_id":        "KEY1234567",
		"private_key":   pemKey,
		"vendor_number": "88001122",
	})
	if err := s.Configure(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Teardown(t.Context()) }()
	s.client.baseURL, s.client.limiter = server.URL, rate.NewLimiter(rate.Inf, 1)
	s.now = func() time.Time { return testTime }

	for _, resName := range []string{"customer_reviews", "analytics_reports", "sales_reports"} {
		prev := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
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
		err = s.ExtractFrom(t.Context(), p.Records(), filament.ExtractOpts{Resources: []string{resName}}, plans)
		p.CloseIngest(err)
		if waitErr := p.Wait(); waitErr != nil {
			t.Fatal(waitErr)
		}
		if len(sink.rows) == 0 {
			t.Fatalf("%s emitted 0 rows", resName)
		}
		lastKey := sink.rows[len(sink.rows)-1].key
		if len(lastKey) != 1 || !strings.HasPrefix(lastKey[0], "2026-11-16T") {
			t.Fatalf("%s unexpected final watermark key: %v", resName, lastKey)
		}
	}
}
