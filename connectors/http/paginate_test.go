package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

const throttleTestManifest = `
version: 1
name: test
display_name: Test
description: Generic HTTP runtime fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: https://example.com
  rate_limit:
    dynamic:
      remaining_header: Budget-Remaining
      reset_header: Budget-Reset
      reset_format: unix_seconds
    responses:
      - status: 403
        header: Retry-After
      - status: 403
        header: Budget-Remaining
        header_value: "0"
      - status: 403
        body_path: error.detail
        body_contains: capacity exhausted
        backoff_seconds: 90
resources:
  - name: items
    path: /items
    records: $
    primary_key: [id]
    fields:
      id: int64
    response:
      poll_pending: true
`

func throttleTestSource(t *testing.T, handler http.HandlerFunc) *Source {
	t.Helper()
	api := httptest.NewServer(handler)
	t.Cleanup(api.Close)
	src := NewManifest([]byte(strings.Replace(throttleTestManifest, "base_url: https://example.com", "base_url: "+api.URL, 1)))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Teardown(context.Background()) })
	return src
}

func throttleTestSpec(t *testing.T) manifest.RateLimit {
	t.Helper()
	m, err := manifest.Parse([]byte(throttleTestManifest))
	if err != nil {
		t.Fatal(err)
	}
	return m.Connection.RateLimit
}

func TestHTTPForbiddenRateLimitRetries(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var calls atomic.Int64
			src := throttleTestSource(t, func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set("Retry-After", "1")
					w.WriteHeader(status)
					fmt.Fprint(w, `{"message":"rate limited"}`)
					return
				}
				fmt.Fprint(w, `[{"id":1}]`)
			})
			var sink collectSink
			var retries atomic.Int64
			if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{
				Resources: []string{"items"},
				Observe: func(p filament.SourceProgress) {
					if p.Kind == filament.SourceProgressRateLimited {
						retries.Add(1)
					}
				},
			}); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 2 || retries.Load() != 1 || len(sink.records) != 1 {
				t.Fatalf("calls=%d retries=%d rows=%d", calls.Load(), retries.Load(), len(sink.records))
			}
		})
	}
}

func TestHTTPForbiddenPermissionsAreFatal(t *testing.T) {
	var calls atomic.Int64
	src := throttleTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":{"detail":"permission denied"}}`)
	})
	var sink collectSink
	err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"items"}})
	if err == nil || !strings.Contains(err.Error(), "403") || calls.Load() != 1 {
		t.Fatalf("calls=%d error=%v", calls.Load(), err)
	}
}

func TestRateLimitDelay(t *testing.T) {
	spec := throttleTestSpec(t)
	limited := []byte(`{"error":{"detail":"Capacity exhausted. Try again later."}}`)
	for _, tc := range []struct {
		name             string
		status           int
		headers          map[string]string
		body             []byte
		attempt          int
		minimum, maximum time.Duration
	}{
		{"server delay exceeds fallback cap", 429, map[string]string{"Retry-After": "120"}, nil, 0, 120 * time.Second, 120 * time.Second},
		{"configured backoff", 403, nil, limited, 0, 90 * time.Second, 90 * time.Second},
		{"configured backoff increases", 403, nil, limited, 2, 6 * time.Minute, 6 * time.Minute},
		{"explicit delay takes precedence", 403, map[string]string{"Retry-After": "2"}, limited, 0, 2 * time.Second, 2 * time.Second},
		{"http date", 429, map[string]string{"Retry-After": time.Now().Add(2 * time.Minute).UTC().Format(http.TimeFormat)}, nil, 0, 119 * time.Second, 120 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tc.status, Header: make(http.Header)}
			for k, v := range tc.headers {
				resp.Header.Set(k, v)
			}
			got, limited := rateLimitDelay(resp, tc.body, tc.attempt, spec)
			if !limited {
				t.Fatal("throttling was not recognized")
			}
			if got < tc.minimum || got > tc.maximum {
				t.Fatalf("delay=%s, want [%s,%s]", got, tc.minimum, tc.maximum)
			}
		})
	}
}

func TestHTTPRateLimitWaitCanBeCancelled(t *testing.T) {
	src := throttleTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	var sink collectSink
	err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"items"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v, want cancellation", err)
	}
}

func TestHTTPNoContentIsEmptyButInvalidJSONFails(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusOK} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			src := throttleTestSource(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) })
			var sink collectSink
			err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"items"}})
			if (err == nil) != (status == http.StatusNoContent) {
				t.Fatalf("status=%d error=%v", status, err)
			}
			if len(sink.records) != 0 {
				t.Fatal("empty response emitted rows")
			}
		})
	}
}

func gongTestSource(t *testing.T, handler http.HandlerFunc) *Source {
	t.Helper()
	api := httptest.NewServer(handler)
	t.Cleanup(api.Close)
	src := catalogSource(t, "gong@v2")
	if err := src.Configure(t.Context(), filament.NewConfig(map[string]any{
		"access_key":        "key",
		"access_key_secret": "secret",
		"base_url":          api.URL,
	})); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Teardown(context.Background()) })
	return src
}

func TestHTTPEmptyResponseRule(t *testing.T) {
	// Gong answers 404 with this body when a filter matches no calls.
	const emptyBody = `{"requestId":"sanitized","errors":["No calls found corresponding to the provided filters"]}`
	for _, tc := range []struct {
		name, body, resource string
		wantEmpty            bool
	}{
		{"matched rule", emptyBody, "calls", true},
		{"unrelated 404", `{"errors":["Resource not found"]}`, "calls", false},
		{"resource without rule", emptyBody, "users", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := gongTestSource(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, tc.body)
			})
			var sink collectSink
			err := src.Extract(t.Context(), &sink, filament.ExtractOpts{Resources: []string{tc.resource}})
			if (err == nil) != tc.wantEmpty || len(sink.records) != 0 {
				t.Fatalf("rows=%d error=%v", len(sink.records), err)
			}
		})
	}
}

func TestHTTPPendingResponseCanBeCancelled(t *testing.T) {
	src := throttleTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{}`)
	})
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	var sink collectSink
	err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"items"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v, want cancellation", err)
	}
	if len(sink.records) != 0 {
		t.Fatal("pending responses must not be emitted")
	}
}

func TestRateLimitMatchingIsManifestScoped(t *testing.T) {
	spec := throttleTestSpec(t)
	for _, tc := range []struct {
		name, body string
		status     int
		headers    map[string]string
		want       bool
	}{
		{"body match", `{"error":{"detail":"CAPACITY EXHAUSTED"}}`, 403, nil, true},
		{"permission denial", `{"error":{"detail":"permission denied"}}`, 403, nil, false},
		{"wrong status", `{"error":{"detail":"capacity exhausted"}}`, 401, nil, false},
		{"wrong body path", `{"message":"capacity exhausted"}`, 403, nil, false},
		{"invalid JSON", `{"error":{"detail":"capacity exhausted"}} trailing`, 403, nil, false},
		{"wrong body type", `{"error":{"detail":{"value":"capacity exhausted"}}}`, 403, nil, false},
		{"header presence", "", 403, map[string]string{"Retry-After": "2"}, true},
		{"exhausted budget", "", 403, map[string]string{"Budget-Remaining": "0"}, true},
		{"available budget", "", 403, map[string]string{"Budget-Remaining": "1"}, false},
		{"unconfigured header", "", 403, map[string]string{"Other-Remaining": "0"}, false},
		{"standard status", "", 429, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tc.status, Header: make(http.Header)}
			for k, v := range tc.headers {
				resp.Header.Set(k, v)
			}
			if _, got := rateLimitDelay(resp, []byte(tc.body), 0, spec); got != tc.want {
				t.Fatalf("configured=%v, want %v", got, tc.want)
			}
			if _, got := rateLimitDelay(resp, []byte(tc.body), 0, manifest.RateLimit{}); got != (tc.status == 429) {
				t.Fatal("unconfigured provider-specific response was retried")
			}
		})
	}
	// Conditions on the same rule are conjunctive.
	rule := manifest.RateLimitResponse{Status: 403, Header: "Budget-Remaining", HeaderValue: "0", BodyPath: "error.detail", BodyContains: "capacity exhausted"}
	resp := &http.Response{StatusCode: 403, Header: http.Header{"Budget-Remaining": []string{"1"}}}
	if matchesRateLimit(resp, []byte(`{"error":{"detail":"capacity exhausted"}}`), rule) {
		t.Fatal("body match bypassed header condition")
	}
}

func TestHTTPBudgetResetUsesConfiguredLimiter(t *testing.T) {
	var calls atomic.Int64
	src := throttleTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Budget-Remaining", "0")
		w.Header().Set("Budget-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
		w.WriteHeader(http.StatusForbidden)
	})
	// The retry backoff expires after one second; the limiter must hold the
	// next request until the budget resets or the caller cancels.
	ctx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
	defer cancel()
	var sink collectSink
	err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"items"}})
	if !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 {
		t.Fatalf("calls=%d error=%v", calls.Load(), err)
	}
}

const linkHeaderTestManifest = `
version: 1
name: test
display_name: Test
description: Generic HTTP runtime fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: https://example.com
resources:
  - name: items
    path: /items
    records: $
    primary_key: [id]
    fields:
      id: int64
    pagination:
      link: next
`

// TestHTTPLinkHeaderWalksRepeatedLinkFields pages an API that puts each Link
// relation in its own Link field — the second one holding rel="next" — rather
// than in a single comma-separated field. Both forms are the same list, so the
// walk must reach page 2 and emit both rows.
func TestHTTPLinkHeaderWalksRepeatedLinkFields(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			fmt.Fprint(w, `[{"id":2}]`)
			return
		}
		base := "http://" + r.Host + "/items"
		w.Header().Add("Link", `<`+base+`?page=9>; rel="last"`)
		w.Header().Add("Link", `<`+base+`?page=2>; rel="next"`)
		fmt.Fprint(w, `[{"id":1}]`)
	}))
	t.Cleanup(api.Close)

	src := NewManifest([]byte(strings.Replace(linkHeaderTestManifest,
		"base_url: https://example.com", "base_url: "+api.URL, 1)))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Teardown(context.Background()) })

	var sink collectSink
	if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{
		Resources: []string{"items"},
	}); err != nil {
		t.Fatal(err)
	}

	var ids []string
	for _, rec := range sink.records {
		ids = append(ids, rec.ID)
	}
	if strings.Join(ids, ",") != "1,2" {
		t.Fatalf("ids=%v, want [1 2] — the rel=\"next\" field after the first was not followed", ids)
	}
}

const lastRecordTestManifest = `
version: 1
name: test
display_name: Test
description: Generic HTTP runtime fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
config:
  api_key:
    type: string
    required: true
  api_token:
    type: secret
    required: true
connection:
  base_url: https://example.com
  auth:
    header:
      name: Authorization
      value: OAuth oauth_consumer_key="{{ config.api_key }}", oauth_token="{{ config.api_token }}"
  headers: {Accept: application/json}
resources:
  - name: items
    path: /items
    query:
      limit: "2"
    pagination:
      last_record:
        field: id
        request: query.before
        page_size: 2
    primary_key: [id]
    fields:
      id: string
    response:
      records: $
`

// TestLastRecordPaginationWalksBeforeCursor pages a root-array API that caps
// at two rows per request and offers no next-page token. The first full page
// must continue with before set to the last record's id, and the short second
// page must end the walk. Every request must carry the templated OAuth
// Authorization header, never the credentials in the URL.
func TestLastRecordPaginationWalksBeforeCursor(t *testing.T) {
	var calls atomic.Int64
	var befores []string
	var authorizations []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		befores = append(befores, r.URL.Query().Get("before"))
		authorizations = append(authorizations, r.Header.Get("Authorization"))
		if got := r.URL.Query().Get("key"); got != "" {
			t.Errorf("request carried key in the URL query: %q", got)
		}
		if got := r.URL.Query().Get("token"); got != "" {
			t.Errorf("request carried token in the URL query: %q", got)
		}
		switch r.URL.Query().Get("before") {
		case "":
			fmt.Fprint(w, `[{"id":"a1"},{"id":"a2"}]`)
		case "a2":
			fmt.Fprint(w, `[{"id":"a3"}]`)
		default:
			fmt.Fprint(w, `[]`)
		}
	}))
	t.Cleanup(api.Close)

	src := NewManifest([]byte(strings.Replace(lastRecordTestManifest,
		"base_url: https://example.com", "base_url: "+api.URL, 1)))
	if err := src.Configure(t.Context(), filament.NewConfig(map[string]any{
		"api_key": "test-key", "api_token": "test-token",
	})); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Teardown(context.Background()) })

	var sink collectSink
	if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{
		Resources: []string{"items"},
	}); err != nil {
		t.Fatal(err)
	}

	if calls.Load() != 2 {
		t.Fatalf("requests=%d, want 2 (one full page plus one short page)", calls.Load())
	}
	if len(befores) != 2 || befores[0] != "" || befores[1] != "a2" {
		t.Fatalf("before params=%v, want [\"\" \"a2\"]", befores)
	}
	const wantAuth = `OAuth oauth_consumer_key="test-key", oauth_token="test-token"`
	for i, got := range authorizations {
		if got != wantAuth {
			t.Fatalf("request %d Authorization=%q, want %q", i, got, wantAuth)
		}
	}
	var ids []string
	for _, rec := range sink.records {
		ids = append(ids, rec.ID)
	}
	if strings.Join(ids, ",") != "a1,a2,a3" {
		t.Fatalf("ids=%v, want [a1 a2 a3]", ids)
	}
}

const scrollTestManifest = `
version: 1
name: test
display_name: Test
description: Generic HTTP runtime fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: https://example.com
  headers: {Accept: application/json}
resources:
  - name: companies
    path: /companies/scroll
    pagination:
      cursor:
        response: scroll_param
        request: query.scroll_param
        stop_on_empty: true
    primary_key: [id]
    fields:
      id: string
    response:
      records: data
`

// scrollAPI mimics a scroll endpoint that keeps returning a cursor on its
// final, empty page and answers 404 when that cursor is followed.
func scrollAPI(t *testing.T, pages map[string]string) (*httptest.Server, *[]string) {
	t.Helper()
	var params []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		param := r.URL.Query().Get("scroll_param")
		params = append(params, param)
		body, ok := pages[param]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"errors":[{"code":"not_found","message":"scroll parameter not found"}]}`)
			return
		}
		fmt.Fprint(w, body)
	}))
	t.Cleanup(api.Close)
	return api, &params
}

func extractScroll(t *testing.T, manifest, baseURL string) ([]string, error) {
	t.Helper()
	src := NewManifest([]byte(strings.Replace(manifest,
		"base_url: https://example.com", "base_url: "+baseURL, 1)))
	if err := src.Configure(t.Context(), filament.NewConfig(map[string]any{})); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Teardown(context.Background()) })
	var sink collectSink
	err := src.Extract(context.Background(), &sink, filament.ExtractOpts{
		Resources: []string{"companies"},
	})
	var ids []string
	for _, rec := range sink.records {
		ids = append(ids, rec.ID)
	}
	return ids, err
}

// The final page has no records but still carries a cursor. With
// stop_on_empty the walk ends there instead of following the cursor into a 404.
func TestCursorStopOnEmptyEndsWalkOnEmptyPage(t *testing.T) {
	api, params := scrollAPI(t, map[string]string{
		"":   `{"data":[{"id":"c1"},{"id":"c2"}],"scroll_param":"s1"}`,
		"s1": `{"data":[{"id":"c3"}],"scroll_param":"s2"}`,
		"s2": `{"data":[],"scroll_param":"s3"}`,
	})
	ids, err := extractScroll(t, scrollTestManifest, api.URL)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := strings.Join(*params, ","); got != ",s1,s2" {
		t.Fatalf("scroll params=%q, want \",s1,s2\" (no request after the empty page)", got)
	}
	if strings.Join(ids, ",") != "c1,c2,c3" {
		t.Fatalf("ids=%v, want [c1 c2 c3]", ids)
	}
}

// A normal walk is unchanged by stop_on_empty: every page has records, and the
// walk ends when the cursor is dropped.
func TestCursorStopOnEmptyKeepsNormalMultiPageWalk(t *testing.T) {
	api, params := scrollAPI(t, map[string]string{
		"":   `{"data":[{"id":"c1"}],"scroll_param":"s1"}`,
		"s1": `{"data":[{"id":"c2"}],"scroll_param":"s2"}`,
		"s2": `{"data":[{"id":"c3"}]}`,
	})
	ids, err := extractScroll(t, scrollTestManifest, api.URL)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := strings.Join(*params, ","); got != ",s1,s2" {
		t.Fatalf("scroll params=%q, want \",s1,s2\"", got)
	}
	if strings.Join(ids, ",") != "c1,c2,c3" {
		t.Fatalf("ids=%v, want [c1 c2 c3]", ids)
	}
}

// Without stop_on_empty, cursor pagination keeps its existing behaviour: an
// empty page with a cursor is followed, so this API's 404 surfaces as an error.
func TestCursorWithoutStopOnEmptyFollowsCursorPastEmptyPage(t *testing.T) {
	api, params := scrollAPI(t, map[string]string{
		"":   `{"data":[{"id":"c1"}],"scroll_param":"s1"}`,
		"s1": `{"data":[],"scroll_param":"s2"}`,
	})
	manifest := strings.Replace(scrollTestManifest, "        stop_on_empty: true\n", "", 1)
	if manifest == scrollTestManifest {
		t.Fatal("fixture did not drop stop_on_empty")
	}
	_, err := extractScroll(t, manifest, api.URL)
	if err == nil {
		t.Fatal("Extract succeeded, want the 404 from following the cursor past the empty page")
	}
	if got := strings.Join(*params, ","); got != ",s1,s2" {
		t.Fatalf("scroll params=%q, want \",s1,s2\" (cursor followed past the empty page)", got)
	}
}

// Strict cursor pagination reads the cursor envelope itself instead of calling
// the paginator, so stop_on_empty must also end the walk there.
func TestStrictCursorStopOnEmptyEndsWalkOnEmptyPage(t *testing.T) {
	api, params := scrollAPI(t, map[string]string{
		"":   `{"data":[{"id":"c1"}],"scroll_param":"s1","has_more":true}`,
		"s1": `{"data":[],"scroll_param":"s2","has_more":true}`,
	})
	manifest := strings.Replace(scrollTestManifest, "        stop_on_empty: true\n",
		"        more: has_more\n        strict: true\n        stop_on_empty: true\n", 1)
	if manifest == scrollTestManifest {
		t.Fatal("fixture did not add strict")
	}
	ids, err := extractScroll(t, manifest, api.URL)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := strings.Join(*params, ","); got != ",s1" {
		t.Fatalf("scroll params=%q, want \",s1\" (no request after the empty page)", got)
	}
	if strings.Join(ids, ",") != "c1" {
		t.Fatalf("ids=%v, want [c1]", ids)
	}
}
