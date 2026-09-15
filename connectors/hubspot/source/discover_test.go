package hubspot

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"golang.org/x/time/rate"

	"github.com/galaxy-io/filament"
)

func TestDiscoverIsolatesUnavailableResources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/schemas"):
			_, _ = w.Write([]byte(`{"results":[{"objectTypeId":"2-123","labels":{"plural":"Vehicles"}}]}`))
		case strings.HasSuffix(req.URL.Path, "/0-5"):
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"category":"MISSING_SCOPES"}`))
		case strings.Contains(req.URL.Path, "/properties/"):
			_, _ = w.Write([]byte(`{"results":[{"name":"lastmodifieddate"},{"name":"hs_lastmodifieddate"},{"name":"custom_text"}]}`))
		default:
			_, _ = w.Write([]byte(`{"results":[]}`))
		}
	}))
	defer server.Close()
	s := New()
	if err := s.Configure(t.Context(), filament.NewConfig(map[string]any{"api_key": "test"})); err != nil {
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
	if found["tickets"].Selectable || found["tickets"].Metadata["unavailable_reason"] == "" {
		t.Fatal("unavailable tickets offered for selection")
	}
	if !found["contacts"].Selectable || found["contacts"].Metadata["default_resources"] != "true" {
		t.Fatal("contacts unavailable")
	}
	if custom := found["custom_objects_2_123"]; !custom.Selectable || custom.DisplayName != "Vehicles" || custom.Metadata["default_resources"] != "false" {
		t.Fatalf("custom object: %+v", custom)
	}
	defaults, err := s.PlanResources(t.Context(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(defaults) != 8 || slices.Contains(defaults, "tickets") || slices.Contains(defaults, "custom_objects_2_123") {
		t.Fatal(defaults)
	}
}

func TestIncrementalRejectsUnsupportedResourcesAndCheckpoints(t *testing.T) {
	s := New()
	if _, err := s.PlanIncremental(t.Context(), []string{"owners"}, nil, nil); err == nil {
		t.Fatal("owners accepted as incremental")
	}
	if _, err := s.PlanIncremental(t.Context(), []string{"contacts"}, map[string]filament.Checkpoint{"contacts": watermarkPlan("deals", testTime)}, nil); err == nil {
		t.Fatal("wrong resource checkpoint accepted")
	}
	if _, err := s.PlanIncremental(t.Context(), []string{"contacts"}, nil, map[string]filament.ResourceCursorConfig{"contacts": {Field: "created_at"}}); err == nil {
		t.Fatal("creation cursor accepted")
	}
	if _, err := s.PlanResources(t.Context(), []string{"custom_objects_2_123/../../contacts"}, nil); err == nil {
		t.Fatal("invalid custom resource accepted")
	}
}
