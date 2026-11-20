package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// isolatingSink is a collectSink that accepts resource failure reports, as the
// engine's inlet does under isolation. quarantine names a resource whose
// writer rejects every row after the first `after`, as a quarantined
// resource's builder does.
type isolatingSink struct {
	collectSink
	quarantine string
	after      int

	mu     sync.Mutex
	failed map[string][]error
}

var _ filament.ResourceFailureReporter = (*isolatingSink)(nil)

func (s *isolatingSink) Builder(resource string, part int, schema rowmodel.Schema) (arrowbatch.RowWriter, error) {
	w, err := s.collectSink.Builder(resource, part, schema)
	if err != nil || resource != s.quarantine {
		return w, err
	}
	return &quarantineWriter{RowWriter: w, resource: resource, after: s.after}, nil
}

func (s *isolatingSink) FailResource(resource string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failed == nil {
		s.failed = map[string][]error{}
	}
	s.failed[resource] = append(s.failed[resource], err)
}

func (s *isolatingSink) recordsFor(resource string) []string {
	var ids []string
	for _, rec := range s.records {
		if rec.Resource == resource {
			ids = append(ids, rec.ID)
		}
	}
	return ids
}

type quarantineWriter struct {
	arrowbatch.RowWriter
	resource string
	after    int
	rows     int
}

func (w *quarantineWriter) EndRow(meta rowmodel.Meta) error {
	if w.rows >= w.after {
		return &filament.ResourceError{Resource: w.resource, Err: fmt.Errorf("%w: sink rejected the resource", filament.ErrResourceQuarantined)}
	}
	w.rows++
	return w.RowWriter.EndRow(meta)
}

// isolationAPI serves good (two pages), denied (deniedStatus), orphans (a
// child of denied), parents p1..p3 and children, where p2's children
// return childStatus.
type isolationAPI struct {
	deniedStatus int
	childStatus  int
	requests     sync.Map // path -> *atomic.Int64
}

func (a *isolationAPI) count(path string) int64 {
	v, ok := a.requests.Load(path)
	if !ok {
		return 0
	}
	return v.(*atomic.Int64).Load()
}

func (a *isolationAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	v, _ := a.requests.LoadOrStore(r.URL.Path, &atomic.Int64{})
	v.(*atomic.Int64).Add(1)
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/good":
		if r.URL.Query().Get("start_cursor") == "" {
			fmt.Fprint(w, `{"items":[{"id":"g1"}],"next_cursor":"2","has_more":true}`)
			return
		}
		fmt.Fprint(w, `{"items":[{"id":"g2"}],"has_more":false}`)
	case "/denied":
		if a.deniedStatus != 0 {
			w.WriteHeader(a.deniedStatus)
			fmt.Fprint(w, `{"message":"no access"}`)
			return
		}
		fmt.Fprint(w, `{"items":[{"id":"d1"}],"has_more":false}`)
	case "/denied/d1/orphans":
		fmt.Fprint(w, `{"items":[{"id":"o1"}],"has_more":false}`)
	case "/parents":
		fmt.Fprint(w, `{"items":[{"id":"p1"},{"id":"p2"},{"id":"p3"}],"has_more":false}`)
	case "/parents/p2/children":
		if a.childStatus != 0 {
			w.WriteHeader(a.childStatus)
			fmt.Fprint(w, `{"message":"gone"}`)
			return
		}
		fmt.Fprint(w, `{"items":[{"id":"c-p2"}],"has_more":false}`)
	default:
		if parent, ok := strings.CutPrefix(r.URL.Path, "/parents/"); ok {
			parent = strings.TrimSuffix(parent, "/children")
			fmt.Fprintf(w, `{"items":[{"id":"c-%s"}],"has_more":false}`, parent)
			return
		}
		http.NotFound(w, r)
	}
}

func writeIsolationManifest(t *testing.T, baseURL string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "isolation.yaml")
	page := `    response:
      records: $.items
      pagination:
        cursor:
          response: next_cursor
          request: query.start_cursor
          more: has_more
`
	data := fmt.Sprintf(`version: 1
name: isolation
display_name: Isolation
description: Test isolation connector.
dark_logo_url: https://cdn.example.com/i-dark.svg
light_logo_url: https://cdn.example.com/i-light.svg
connection:
  base_url: %[1]s
resources:
  - name: good
    path: /good
    primary_key: [id]
%[2]s  - name: denied
    path: /denied
    primary_key: [id]
    capture:
      id: id
%[2]s  - name: orphans
    path: /denied/{parent}/orphans
    params:
      parent: parent.id
    for_each: denied
    primary_key: [id]
%[2]s  - name: parents
    path: /parents
    primary_key: [id]
    capture:
      id: id
%[2]s  - name: children
    path: /parents/{parent}/children
    params:
      parent: parent.id
    for_each: parents
    parent: {concurrency: 1}
    primary_key: [id]
%[2]s`, baseURL, page)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func isolationSource(t *testing.T, api *isolationAPI) *Source {
	t.Helper()
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	src := New()
	if err := src.Configure(context.Background(), filament.NewConfig(map[string]any{
		"manifest_path": writeIsolationManifest(t, server.URL),
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	t.Cleanup(func() { _ = src.Teardown(context.Background()) })
	return src
}

func TestIsolationContainsEndpointFailures(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			api := &isolationAPI{deniedStatus: status, childStatus: http.StatusNotFound}
			src := isolationSource(t, api)
			sink := &isolatingSink{}
			if err := src.Extract(context.Background(), sink, filament.ExtractOpts{}); err != nil {
				t.Fatalf("extract under isolation: %v", err)
			}
			if got := sink.recordsFor("good"); strings.Join(got, ",") != "g1,g2" {
				t.Fatalf("good records = %v, want both pages", got)
			}
			if got := sink.recordsFor("parents"); len(got) != 3 {
				t.Fatalf("parents records = %v, want 3", got)
			}
			if len(sink.failed) != 3 {
				t.Fatalf("failed resources = %v, want denied, orphans and children", sink.failed)
			}
			denied := sink.failed["denied"]
			if len(denied) != 1 || !strings.Contains(denied[0].Error(), fmt.Sprintf("HTTP %d", status)) {
				t.Fatalf("denied failures = %v, want one HTTP %d", denied, status)
			}
			orphans := sink.failed["orphans"]
			if len(orphans) != 1 || !strings.Contains(orphans[0].Error(), `parent resource "denied" did not complete`) {
				t.Fatalf("orphans failures = %v, want one parent failure", orphans)
			}
			if api.count("/denied/d1/orphans") != 0 {
				t.Fatal("orphans read from a failed parent")
			}
			children := sink.failed["children"]
			if len(children) != 1 || !strings.Contains(children[0].Error(), "HTTP 404") {
				t.Fatalf("children failures = %v, want one HTTP 404", children)
			}
			// Concurrency 1 reads parents in order: p2 fails the child as a
			// whole, so p3 is never requested.
			if api.count("/parents/p1/children") != 1 || api.count("/parents/p3/children") != 0 {
				t.Fatalf("child requests p1=%d p3=%d, want 1 and 0", api.count("/parents/p1/children"), api.count("/parents/p3/children"))
			}
		})
	}
}

func TestIsolationUnauthorizedFailsTheRun(t *testing.T) {
	api := &isolationAPI{deniedStatus: http.StatusUnauthorized}
	src := isolationSource(t, api)
	sink := &isolatingSink{}
	err := src.Extract(context.Background(), sink, filament.ExtractOpts{})
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("extract err = %v, want HTTP 401", err)
	}
	if len(sink.failed["denied"]) != 0 {
		t.Fatalf("401 was reported as a resource failure: %v", sink.failed)
	}
}

func TestIsolationStrictModeFailsOnEndpointFailure(t *testing.T) {
	api := &isolationAPI{deniedStatus: http.StatusForbidden}
	src := isolationSource(t, api)
	var sink collectSink
	err := src.Extract(context.Background(), &sink, filament.ExtractOpts{})
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") || !strings.Contains(err.Error(), "(2/3 resources succeeded)") {
		t.Fatalf("strict extract err = %v, want the HTTP 403 to fail the run", err)
	}
	if api.count("/parents/p1/children") != 0 {
		t.Fatal("strict mode read children after a top-level failure")
	}

	api = &isolationAPI{childStatus: http.StatusNotFound}
	src = isolationSource(t, api)
	sink = collectSink{}
	err = src.Extract(context.Background(), &sink, filament.ExtractOpts{})
	if err == nil || !strings.Contains(err.Error(), "child resource extraction failed") || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("strict extract err = %v, want the child HTTP 404 to fail the run", err)
	}
}

func TestIsolationChildFailureForOneParent(t *testing.T) {
	api := &isolationAPI{childStatus: http.StatusForbidden}
	src := isolationSource(t, api)
	sink := &isolatingSink{}
	if err := src.Extract(context.Background(), sink, filament.ExtractOpts{Resources: []string{"children", "good"}}); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(sink.failed) != 1 || len(sink.failed["children"]) != 1 {
		t.Fatalf("failed = %v, want children once", sink.failed)
	}
	if err := sink.failed["children"][0]; !strings.Contains(err.Error(), "parent map[id:p2]") || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("children cause = %v", err)
	}
	if got := sink.recordsFor("good"); len(got) != 2 {
		t.Fatalf("good records = %v, want 2", got)
	}
}

func TestIsolationQuarantinedResourceStopsQuietly(t *testing.T) {
	api := &isolationAPI{}
	src := isolationSource(t, api)
	sink := &isolatingSink{quarantine: "good"}
	if err := src.Extract(context.Background(), sink, filament.ExtractOpts{}); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(sink.failed) != 0 {
		t.Fatalf("a quarantined resource was reported again: %v", sink.failed)
	}
	if api.count("/good") != 1 {
		t.Fatalf("good requests = %d, want the read to stop after the first page", api.count("/good"))
	}
	if got := sink.recordsFor("children"); len(got) != 3 {
		t.Fatalf("children records = %v, want 3", got)
	}
}

func TestIsolationQuarantinedParentFailsItsChildren(t *testing.T) {
	api := &isolationAPI{}
	src := isolationSource(t, api)
	sink := &isolatingSink{quarantine: "parents", after: 1}
	if err := src.Extract(context.Background(), sink, filament.ExtractOpts{}); err != nil {
		t.Fatalf("extract: %v", err)
	}
	children := sink.failed["children"]
	if len(sink.failed) != 1 || len(children) != 1 {
		t.Fatalf("failed = %v, want only children", sink.failed)
	}
	if !errors.Is(children[0], filament.ErrResourceQuarantined) || !strings.Contains(children[0].Error(), `parent resource "parents" did not complete`) {
		t.Fatalf("children cause = %v", children[0])
	}
	for _, path := range []string{"/parents/p1/children", "/parents/p2/children", "/parents/p3/children"} {
		if api.count(path) != 0 {
			t.Fatalf("%s read from a partial parent capture set", path)
		}
	}
	if got := sink.recordsFor("orphans"); len(got) != 1 {
		t.Fatalf("orphans records = %v, want 1", got)
	}
}

func TestIsolationClassification(t *testing.T) {
	for _, tc := range []struct {
		err   error
		scope bool
	}{
		{&statusError{status: 403}, true},
		{fmt.Errorf("wrap: %w", &statusError{status: 404}), true},
		{&statusError{status: 422}, true},
		{&statusError{status: 401}, false},
		{&statusError{status: 429}, false},
		{&statusError{status: 500}, false},
		{&statusError{status: 503}, false},
		{resourceScoped(errors.New("decode")), true},
		{errors.New("http: dial tcp: connection refused"), false},
		{context.Canceled, false},
	} {
		if got := isResourceScoped(tc.err); got != tc.scope {
			t.Errorf("isResourceScoped(%v) = %v, want %v", tc.err, got, tc.scope)
		}
	}
}
