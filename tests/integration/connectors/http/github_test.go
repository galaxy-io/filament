//go:build integration

package http_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	httpapi "github.com/galaxy-io/filament/connectors/http"
	pgsink "github.com/galaxy-io/filament/connectors/postgres/sink"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

// Exercise the shipped GitHub schemas through Arrow and a real destination.
// Membership snapshots remove missing users; traffic upserts retain older days.
func TestGitHubPostgresSync(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	pg := testcontainers.Postgres(t)
	var phase atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/orgs/test-org/repos":
			fmt.Fprint(w, `[{"id":10,"node_id":"R_10","name":"one","full_name":"test-org/one","private":false,"fork":false,"archived":false,"disabled":false,"html_url":"https://github.com/test-org/one"}]`)
		case "/repos/test-org/one/stargazers":
			switch phase.Load() {
			case 0:
				fmt.Fprint(w, `[{"starred_at":"2026-09-01T00:00:00Z","user":{"id":1,"login":"alice"}},{"starred_at":"2026-09-02T00:00:00Z","user":{"id":2,"login":"bob"}}]`)
			case 1:
				fmt.Fprint(w, `[{"starred_at":"2026-09-02T00:00:00Z","user":{"id":2,"login":"bob"}}]`)
			default:
				fmt.Fprint(w, `[]`)
			}
		case "/repos/test-org/one/traffic/views":
			if phase.Load() == 0 {
				fmt.Fprint(w, `{"views":[{"timestamp":"2026-08-01T00:00:00Z","count":5,"uniques":2},{"timestamp":"2026-09-01T00:00:00Z","count":10,"uniques":3}]}`)
			} else {
				fmt.Fprint(w, `{"views":[{"timestamp":"2026-09-01T00:00:00Z","count":12,"uniques":4}]}`)
			}
		case "/repos/test-org/one/issues":
			fmt.Fprint(w, `[{"id":100,"number":1}]`)
		case "/repos/test-org/one/issues/1/timeline":
			fmt.Fprint(w, `[{"event":"committed","sha":"abc"},{"event":"cross-referenced","source":{"type":"issue"}}]`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	manifest, err := os.ReadFile("../../../../connectors/http/manifests/github.yaml")
	if err != nil {
		t.Fatal(err)
	}
	src := httpapi.NewManifest([]byte(strings.Replace(string(manifest), "https://api.github.com", api.URL, 1)))
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"organization": "test-org", "token": "test-token"})); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(ctx)

	syncResource := func(resource string, mode filament.IngestionType) {
		t.Helper()
		schema, err := src.Schema(ctx, resource)
		if err != nil {
			t.Fatal(err)
		}
		var rows testutil.CollectSink
		defer rows.Release()
		if err := src.Extract(ctx, &rows, filament.ExtractOpts{Resources: []string{resource}}); err != nil {
			t.Fatal(err)
		}
		policy := filament.WritePolicyForIngestion(mode)
		policy.Resource, policy.Keys = resource, schema.PrimaryKey
		dst := pgsink.New()
		run := filament.RunSpec{
			Run:       filament.RunID(fmt.Sprintf("github-%s-%d", resource, phase.Load())),
			Resources: []string{resource}, Sink: filament.Ref{Config: map[string]any{"dsn": pg.DSN(), "schema": "github"}},
			WritePolicies: map[string]filament.WritePolicy{resource: policy},
		}
		if err := dst.Open(ctx, run); err != nil {
			t.Fatal(err)
		}
		defer dst.Abort(ctx)
		if err := dst.EnsureSchema(ctx, resource, schema); err != nil {
			t.Fatal(err)
		}
		if err := testutil.ApplyAll(ctx, dst, rows.BatchesFor(resource), policy); err != nil {
			t.Fatal(err)
		}
		if err := dst.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	assertInt := func(query string, want int) {
		t.Helper()
		var got int
		if err := pg.Pool().QueryRow(ctx, query).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s = %d, want %d", query, got, want)
		}
	}
	for i, want := range []int{2, 1, 0} {
		phase.Store(int32(i))
		syncResource("stargazers", filament.IngestionFullReplace)
		assertInt("SELECT count(*) FROM github.stargazers", want)
		if i == 1 {
			assertInt("SELECT user_id FROM github.stargazers", 2)
		}
		if i < 2 {
			syncResource("traffic_views", filament.IngestionFullUpsert)
			assertInt("SELECT count(*) FROM github.traffic_views", 2)
		}
	}
	assertInt("SELECT sum(count) FROM github.traffic_views", 17)
	syncResource("issue_timeline", filament.IngestionFullReplace)
	syncResource("issue_timeline", filament.IngestionFullReplace)
	assertInt("SELECT count(*) FROM github.issue_timeline", 2)
}
