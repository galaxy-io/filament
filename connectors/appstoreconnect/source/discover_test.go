package appstoreconnect

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/galaxy-io/filament"
)

func TestDiscoverIsolatesUnavailableResources(t *testing.T) {
	pemKey, _ := generateTestP8Key(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case strings.HasPrefix(req.URL.Path, "/v1/apps/app-1/analyticsReportRequests"):
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":[{"id":"e1","code":"FORBIDDEN"}]}`))
		default:
			_, _ = w.Write([]byte(`{"data":[{"id":"app-1","type":"apps","attributes":{"name":"Test"}}]}`))
		}
	}))
	defer server.Close()
	s := New()
	cfg := filament.NewConfig(map[string]any{
		"issuer_id":   "issuer-1",
		"key_id":      "KEY1234567",
		"private_key": pemKey,
	})
	if err := s.Configure(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Teardown(t.Context()) }()
	s.client.baseURL, s.client.limiter = server.URL, rate.NewLimiter(rate.Inf, 1)
	catalog, err := s.Discover(t.Context(), filament.DiscoverOpts{})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]filament.Resource{}
	for _, r := range catalog.Resources {
		found[r.Name] = r
	}
	if !found["apps"].Selectable || !found["customer_reviews"].Selectable {
		t.Fatal("expected apps and customer_reviews to be selectable")
	}
	if found["analytics_reports"].Selectable || found["analytics_reports"].Metadata["unavailable_reason"] == "" {
		t.Fatal("expected analytics_reports to be marked unavailable on 403")
	}
	if found["sales_reports"].Selectable || found["sales_reports"].Metadata["unavailable_reason"] == "" {
		t.Fatal("expected sales_reports to be unavailable when vendor_number is omitted")
	}
	defaults, err := s.PlanResources(t.Context(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(defaults, []string{"analytics_reports", "apps", "customer_reviews"}) {
		t.Fatalf("unexpected defaults: %v", defaults)
	}
}

func TestIncrementalRejectsUnsupportedResourcesAndCheckpoints(t *testing.T) {
	s := New()
	if _, err := s.PlanIncremental(t.Context(), []string{"apps"}, nil, nil); err == nil {
		t.Fatal("apps accepted as incremental")
	}
	if _, err := s.PlanIncremental(t.Context(), []string{"customer_reviews"}, map[string]filament.Checkpoint{"customer_reviews": watermarkPlan("sales_reports", time.Now())}, nil); err == nil {
		t.Fatal("mismatched checkpoint accepted")
	}
	if _, err := s.PlanIncremental(t.Context(), []string{"customer_reviews"}, nil, map[string]filament.ResourceCursorConfig{"customer_reviews": {Field: "wrong_field"}}); err == nil {
		t.Fatal("invalid cursor field accepted")
	}
}

func TestPlanIncrementalResetsWatermarkWhenAppIDsChange(t *testing.T) {
	s := New()
	s.appIDs = []string{"app-1"}
	savedTime := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	cp1 := watermarkPlanWithApps("customer_reviews", savedTime, "app-1")

	// Same app_ids preserves saved watermark.
	plansSame, err := s.PlanIncremental(t.Context(), []string{"customer_reviews"}, map[string]filament.Checkpoint{"customer_reviews": cp1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	markSame, err := planWatermark("customer_reviews", plansSame["customer_reviews"])
	if err != nil || !markSame.Equal(savedTime) {
		t.Fatalf("expected preserved watermark %v, got %v (err=%v)", savedTime, markSame, err)
	}

	// Adding app-2 resets watermark to initialWatermark so app-2 is backfilled from the beginning.
	s.appIDs = []string{"app-1", "app-2"}
	plansChanged, err := s.PlanIncremental(t.Context(), []string{"customer_reviews"}, map[string]filament.Checkpoint{"customer_reviews": cp1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	markChanged, err := planWatermark("customer_reviews", plansChanged["customer_reviews"])
	if err != nil || !markChanged.Equal(initialWatermark) {
		t.Fatalf("expected reset watermark %v on app_ids change, got %v (err=%v)", initialWatermark, markChanged, err)
	}
}
