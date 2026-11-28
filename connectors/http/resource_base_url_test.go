package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/galaxy-io/filament"
)

const resourceBaseURLManifest = `version: 1
name: hosts
display_name: Hosts
description: Per-resource host fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
config:
  region: {type: enum, enum: [us, eu], default: us}
  key: {type: string, default: secret}
connection:
  base_url: https://unused.example.com
  base_url_field: region
  base_urls: {us: MAIN, eu: https://eu.invalid}
  auth: {header: {name: X-Key, value: config.key}}
resources:
  - name: profiles
    path: /profiles
    fields: {id: string}
    response: {records: $}
  - name: events
    path: /export
    base_urls: {us: DATA, eu: https://data-eu.invalid}
    fields: {id: string}
    response: {records: $}
  - name: annotations
    path: /annotations
    base_url: '{{ config.annotations_host | default "DATA" }}'
    fields: {id: string}
    response: {records: $}
`

// hostLog records the requests one host saw. Resources can run concurrently,
// so handlers append under a lock.
type hostLog struct {
	mu   sync.Mutex
	seen []string
}

// String joins the recorded requests in arrival order.
func (l *hostLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.seen, ",")
}

// hostServer answers every path with one record named after the server and
// records the path and authentication it saw.
func hostServer(t *testing.T, name string, seen *hostLog) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.mu.Lock()
		seen.seen = append(seen.seen, r.URL.Path+" key="+r.Header.Get("X-Key"))
		seen.mu.Unlock()
		fmt.Fprintf(w, `[{"id":%q}]`, name)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestResourceBaseURLRoutesEachResourceToItsHost(t *testing.T) {
	var mainSeen, dataSeen hostLog
	mainAPI := hostServer(t, "main", &mainSeen)
	dataAPI := hostServer(t, "data", &dataSeen)
	data := strings.NewReplacer("MAIN", mainAPI.URL, "DATA", dataAPI.URL).Replace(resourceBaseURLManifest)

	src := NewManifest([]byte(data))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(t.Context())
	sink := &collectSink{}
	if err := src.Extract(t.Context(), sink, filament.ExtractOpts{Resources: []string{"profiles", "events", "annotations"}}); err != nil {
		t.Fatal(err)
	}

	if got := mainSeen.String(); got != "/profiles key=secret" {
		t.Fatalf("main host saw %q, want only the profiles request", got)
	}
	got := dataSeen.String()
	for _, want := range []string{"/export key=secret", "/annotations key=secret"} {
		if !strings.Contains(got, want) {
			t.Fatalf("data host saw %q, want %q", got, want)
		}
	}
	if len(sink.records) != 3 {
		t.Fatalf("records = %d, want 3", len(sink.records))
	}
}

func TestResourceBaseURLsFollowTheConnectionSelection(t *testing.T) {
	var euSeen hostLog
	euData := hostServer(t, "eu", &euSeen)
	euMain := hostServer(t, "eu-main", &hostLog{})
	data := strings.NewReplacer("MAIN", "https://main.invalid", "https://eu.invalid", euMain.URL, "https://data-eu.invalid", euData.URL, "DATA", euData.URL).Replace(resourceBaseURLManifest)

	src := NewManifest([]byte(data))
	if err := src.Configure(t.Context(), filament.NewConfig(map[string]any{"region": "eu"})); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(t.Context())
	if err := src.Extract(t.Context(), &collectSink{}, filament.ExtractOpts{Resources: []string{"events"}}); err != nil {
		t.Fatal(err)
	}
	if got := euSeen.String(); got != "/export key=secret" {
		t.Fatalf("eu data host saw %q", got)
	}
}

func TestResourceBaseURLValidation(t *testing.T) {
	valid := strings.NewReplacer("MAIN", "https://main.example.com", "DATA", "https://data.example.com").Replace(resourceBaseURLManifest)
	if err := NewManifest([]byte(valid)).manifestErr; err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	for _, tc := range []struct{ name, old, new, want string }{
		{"missing region", "base_urls: {us: https://data.example.com, eu: https://data-eu.invalid}", "base_urls: {us: https://data.example.com}", "missing a host for eu"},
		{"unknown region", "eu: https://data-eu.invalid}", "eu: https://data-eu.invalid, ap: https://ap.invalid}", "is not a connection.base_urls key"},
		{"both forms", "    base_urls: {us: https://data.example.com", "    base_url: https://data.example.com\n    base_urls: {us: https://data.example.com", "cannot be combined with base_urls"},
		{"cursor scope", `'{{ config.annotations_host | default "https://data.example.com" }}'`, `'{{ cursor }}'`, "base_url"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := strings.Replace(valid, tc.old, tc.new, 1)
			if invalid == valid {
				t.Fatalf("replacement %q did not apply", tc.old)
			}
			err := NewManifest([]byte(invalid)).manifestErr
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}

	withoutField := strings.Replace(valid, "  base_url_field: region\n  base_urls: {us: https://main.example.com, eu: https://eu.invalid}\n", "", 1)
	if err := NewManifest([]byte(withoutField)).manifestErr; err == nil || !strings.Contains(err.Error(), "requires connection.base_url_field") {
		t.Fatalf("error = %v, want base_url_field requirement", err)
	}
}

func TestResourceBaseURLMustStayOnTheConnectionDomain(t *testing.T) {
	for _, tc := range []struct{ name, main, data, want string }{
		{"subdomain", "https://mixpanel.com", "https://data.mixpanel.com", ""},
		{"regional subdomain", "https://eu.mixpanel.com", "https://data-eu.mixpanel.com", ""},
		{"same host", "https://api.example.com", "https://API.example.com/v2", ""},
		{"plain http", "https://mixpanel.com", "http://data.mixpanel.com", "must use https"},
		{"other domain", "https://mixpanel.com", "https://collector.example.net", "is not on the connection's domain"},
		{"lookalike", "https://mixpanel.com", "https://mixpanel.com.evil.io", "is not on the connection's domain"},
		{"public suffix siblings", "https://a.github.io", "https://b.github.io", "is not on the connection's domain"},
		{"loopback with remote connection", "https://mixpanel.com", "http://127.0.0.1:8080", "must use https"},
		{"userinfo", "https://mixpanel.com", "https://user:pass@data.mixpanel.com", "without userinfo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := strings.NewReplacer("MAIN", tc.main, "DATA", tc.data).Replace(resourceBaseURLManifest)
			src := NewManifest([]byte(data))
			err := src.Configure(t.Context(), filament.NewConfig(nil))
			if err == nil {
				defer src.Teardown(t.Context())
			}
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Configure: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestResourceBaseURLFromConfigIsChecked(t *testing.T) {
	data := strings.NewReplacer("MAIN", "https://mixpanel.com", "DATA", "https://data.mixpanel.com").Replace(resourceBaseURLManifest)
	src := NewManifest([]byte(data))
	err := src.Configure(t.Context(), filament.NewConfig(map[string]any{"annotations_host": "https://attacker.example"}))
	if err == nil {
		src.Teardown(t.Context())
	}
	if err == nil || !strings.Contains(err.Error(), "resource annotations") {
		t.Fatalf("error = %v, want the templated annotations host rejected", err)
	}
}
