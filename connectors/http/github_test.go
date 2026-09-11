package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/request"
)

var githubExpectedResources = []string{
	"repositories", "issues", "pull_requests", "repository_details", "stargazers", "watchers", "star_history", "star_count", "forks",
	"issue_comments", "pull_request_review_comments", "pull_request_reviews", "labels", "milestones", "releases", "release_assets", "issue_events",
	"issue_reactions", "issue_comment_reactions", "pull_request_review_comment_reactions",
	"branches", "tags", "commits", "contributors", "languages", "members", "teams", "team_members", "collaborators", "workflows", "workflow_runs", "workflow_jobs", "deployments", "deployment_statuses", "traffic_views", "traffic_clones", "traffic_referrers", "traffic_paths",
	"projects", "project_fields", "project_items", "project_item_field_values", "dependabot_alerts", "code_scanning_alerts", "secret_scanning_alerts",
	"discussions", "discussion_comments", "discussion_replies",
	"pull_request_files", "repository_events", "commit_activity", "code_frequency", "contributor_statistics", "participation", "punch_card", "anonymous_contributors",
	"issue_timeline",
}

func githubRepository(id int, name string) map[string]any {
	return map[string]any{
		"id": id, "node_id": fmt.Sprint(id), "name": name, "full_name": "test-org/" + name,
		"private": false, "fork": false, "archived": false, "disabled": false,
		"html_url":         "https://github.com/test-org/" + name,
		"stargazers_count": 2, "forks_count": 1, "open_issues_count": 0, "size": 42,
	}
}

func githubTestSource(t *testing.T, handler http.HandlerFunc) *Source {
	t.Helper()
	api := httptest.NewServer(handler)
	t.Cleanup(api.Close)
	src := NewManifest([]byte(strings.Replace(string(githubManifest), "https://api.github.com", api.URL, 1)))
	if err := src.Configure(context.Background(), filament.NewConfig(map[string]any{
		"organization": "test-org", "token": "test-token",
	})); err != nil {
		t.Fatal(err)
	}
	src.connector.limiter = request.NewStaticLimiter(0)
	t.Cleanup(func() { _ = src.Teardown(context.Background()) })
	return src
}

func githubRows(t *testing.T, sink *collectSink, resource string) []map[string]any {
	t.Helper()
	var rows []map[string]any
	for _, rec := range sink.records {
		if rec.Resource != resource {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal(rec.Data, &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}

func TestGitHubCommunity(t *testing.T) {
	var calls atomic.Int64
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
			t.Error("missing GitHub request settings")
		}
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		if path == "/orgs/test-org/repos" {
			if r.URL.Query().Get("page") == "2" {
				_ = json.NewEncoder(w).Encode([]any{githubRepository(20, "two")})
			} else {
				w.Header().Set("Link", "<http://"+r.Host+path+"?per_page=100&page=2>; rel=\"next\"")
				_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
			}
			return
		}
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 3 || parts[0] != "repos" || parts[1] != "test-org" || !slices.Contains([]string{"one", "two"}, parts[2]) {
			t.Errorf("unexpected path %s", path)
			http.NotFound(w, r)
			return
		}
		switch {
		case len(parts) == 3:
			repo := githubRepository(10, parts[2])
			if parts[2] == "two" {
				repo["id"] = 20
			}
			repo["subscribers_count"] = 7
			_ = json.NewEncoder(w).Encode(repo)
		case strings.HasSuffix(path, "/stargazers/history"):
			if r.URL.Query().Get("per_page") != "30" {
				t.Error("history page size must be 30")
			}
			fmt.Fprint(w, `[{"week":1754784000,"total":2,"days":[0,2,0,0,0,0,0]}]`)
		case strings.HasSuffix(path, "/stargazers/count"):
			fmt.Fprint(w, `{"count":2}`)
		case strings.HasSuffix(path, "/stargazers"):
			if r.Header.Get("Accept") != "application/vnd.github.star+json" {
				t.Error("missing timestamp media type")
			}
			if r.URL.Query().Get("page") != "2" {
				w.Header().Set("Link", "<http://"+r.Host+path+"?per_page=100&page=2>; rel=\"next\"")
				fmt.Fprint(w, `[{"starred_at":"2026-09-01T12:00:00Z","user":{"id":1,"login":"alice","extra":"preserved"}}]`)
			} else {
				fmt.Fprint(w, `[{"starred_at":"2026-09-02T12:00:00Z","user":{"id":2,"login":"bob"}}]`)
			}
		case strings.HasSuffix(path, "/subscribers"):
			if r.Header.Get("Accept") != "application/vnd.github+json" {
				t.Error("star header leaked into watchers")
			}
			fmt.Fprint(w, `[{"id":1,"login":"alice"}]`)
		case strings.HasSuffix(path, "/forks"):
			if r.URL.Query().Get("sort") != "oldest" {
				t.Error("fork sort missing")
			}
			fmt.Fprint(w, `[{"id":100,"name":"fork","full_name":"alice/fork","owner":{"id":1,"login":"alice"}}]`)
		default:
			t.Errorf("unexpected path %s", path)
			http.NotFound(w, r)
		}
	})
	var sink collectSink
	resources := []string{"repositories", "repository_details", "stargazers", "watchers", "star_history", "star_count", "forks"}
	if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: resources}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 16 {
		t.Fatalf("calls = %d, want 16 paginated requests", calls.Load())
	}
	for _, name := range resources {
		rows := githubRows(t, &sink, name)
		want := 2
		if name == "stargazers" {
			want = 4
		}
		if len(rows) != want {
			t.Fatalf("%s rows=%d, want %d", name, len(rows), want)
		}
		schema, err := src.Schema(context.Background(), name)
		if err != nil {
			t.Fatal(err)
		}
		keys := map[string]bool{}
		for _, row := range rows {
			key := map[string]any{}
			for _, field := range schema.PrimaryKey {
				key[field] = row[field]
			}
			encoded, _ := json.Marshal(key)
			if keys[string(encoded)] {
				t.Fatalf("%s lost repository identity", name)
			}
			keys[string(encoded)] = true
		}
	}
	for _, row := range githubRows(t, &sink, "repository_details") {
		if row["subscribers_count"] != float64(7) || row["stargazers_count"] != float64(2) {
			t.Fatal("watcher and star metrics conflated")
		}
	}
	for _, row := range githubRows(t, &sink, "stargazers") {
		if row["user_id"] == float64(1) {
			raw := row["raw"].(map[string]any)
			if raw["user"].(map[string]any)["extra"] != "preserved" {
				t.Fatal("unprojected user field lost")
			}
		}
	}
}

func TestGitHubStargazersRejectUnidentifiableRows(t *testing.T) {
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/orgs/") {
			_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
		} else {
			fmt.Fprint(w, `[{"starred_at":"2026-09-01T12:00:00Z","user":null}]`)
		}
	})
	var sink collectSink
	err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"stargazers"}})
	if err == nil || !strings.Contains(err.Error(), "user_id") {
		t.Fatalf("error=%v, want missing identity error", err)
	}
	if len(sink.records) != 0 {
		t.Fatal("emitted unidentifiable star")
	}
}

func TestGitHubCollaboration(t *testing.T) {
	responses := map[string]string{
		"/issues":                       `[{"id":30,"number":7}]`,
		"/pulls":                        `[{"id":31,"number":8}]`,
		"/issues/comments":              `[{"id":40,"issue_url":"https://api.github.com/repos/test-org/one/issues/7","created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-02T00:00:00Z","body":"comment"}]`,
		"/pulls/comments":               `[{"id":41,"pull_request_url":"https://api.github.com/repos/test-org/one/pulls/8","pull_request_review_id":50,"created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-02T00:00:00Z","line":null,"original_line":4}]`,
		"/pulls/8/reviews":              `[{"id":50,"state":"APPROVED","submitted_at":"2026-09-02T00:00:00Z"}]`,
		"/labels":                       `[{"id":60,"name":"bug","color":"ffffff"}]`,
		"/milestones":                   `[{"id":70,"number":1,"title":"v1","state":"closed","created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-02T00:00:00Z"}]`,
		"/releases":                     `[{"id":80,"tag_name":"v1","draft":false,"prerelease":false,"created_at":"2026-09-01T00:00:00Z","assets":[{"id":90}]}]`,
		"/releases/80/assets":           `[{"id":90,"name":"app.zip","size":1024,"download_count":42,"created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-02T00:00:00Z"}]`,
		"/issues/events":                `[{"id":100,"event":"closed","created_at":"2026-09-02T00:00:00Z","actor":null,"issue":{"id":30}}]`,
		"/issues/7/reactions":           `[{"id":110,"content":"+1","user":null,"created_at":"2026-09-02T00:00:00Z"}]`,
		"/issues/comments/40/reactions": `[{"id":120,"content":"heart","created_at":"2026-09-02T00:00:00Z"}]`,
		"/pulls/comments/41/reactions":  `[{"id":130,"content":"rocket","created_at":"2026-09-02T00:00:00Z"}]`,
	}
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/orgs/test-org/repos" {
			_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/repos/test-org/one")
		if path == "/milestones" || path == "/issues" || path == "/pulls" {
			if r.URL.Query().Get("state") != "all" {
				t.Errorf("%s must include closed records", path)
			}
		}
		if r.URL.Query().Get("since") != "" {
			t.Error("full scans must not filter historical reaction parents")
		}
		if payload, ok := responses[path]; ok {
			fmt.Fprint(w, payload)
		} else {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	selected := []string{"issue_comments", "pull_request_review_comments", "pull_request_reviews", "labels", "milestones", "releases", "release_assets", "issue_events", "issue_reactions", "issue_comment_reactions", "pull_request_review_comment_reactions"}
	var sink collectSink
	if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: selected}); err != nil {
		t.Fatal(err)
	}
	if len(sink.records) != len(selected) {
		t.Fatalf("rows=%d want %d selected resources", len(sink.records), len(selected))
	}
	for _, name := range selected {
		rows := githubRows(t, &sink, name)
		if len(rows) != 1 || rows[0]["repository_id"] != float64(10) || rows[0]["repository"] != "one" {
			t.Fatalf("%s lost inherited repository scope", name)
		}
	}
	for resource, key := range map[string]string{"pull_request_reviews": "pull_request_id", "release_assets": "release_id", "issue_reactions": "issue_id", "issue_comment_reactions": "comment_id", "pull_request_review_comment_reactions": "comment_id"} {
		if githubRows(t, &sink, resource)[0][key] == nil {
			t.Fatalf("%s missing %s", resource, key)
		}
	}
	row := githubRows(t, &sink, "release_assets")[0]
	if row["download_count"] != float64(42) {
		t.Fatal("asset download count lost")
	}
}

func TestGitHubCommentsIncremental(t *testing.T) {
	for _, resource := range []string{"issue_comments", "pull_request_review_comments"} {
		t.Run(resource, func(t *testing.T) {
			var pages atomic.Int64
			src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/orgs/test-org/repos" {
					_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
					return
				}
				pages.Add(1)
				q := r.URL.Query()
				if q.Get("since") != "2026-09-01T23:55:00Z" || q.Get("sort") != "updated" || q.Get("direction") != "asc" {
					t.Errorf("unexpected incremental query %s", r.URL.RawQuery)
				}
				id := 1
				if q.Get("page") == "2" {
					id = 2
				} else {
					q.Set("page", "2")
					w.Header().Set("Link", "<http://"+r.Host+r.URL.Path+"?"+q.Encode()+">; rel=\"next\"")
				}
				fmt.Fprintf(w, `[{"id":%d,"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-09-03T00:00:00Z","issue_url":"https://api.github.com/repos/test-org/one/issues/7","pull_request_url":"https://api.github.com/repos/test-org/one/pulls/8"}]`, id)
			})
			prev := map[string]filament.Checkpoint{resource: checkpoint.KeysetCheckpoint{
				Mode: checkpoint.ModeIncremental, Cols: []string{resource + "_updated_at"}, Types: []string{"timestamptz"}, Shards: []checkpoint.KeysetShard{{Key: []string{"2026-09-02T00:00:00Z"}}},
			}.ToCheckpoint(resource)}
			ctx := context.Background()
			plan, err := src.PlanIncremental(ctx, []string{resource}, prev, nil)
			if err != nil {
				t.Fatal(err)
			}
			var sink collectSink
			if err := src.ExtractFrom(ctx, &sink, filament.ExtractOpts{Resources: []string{resource}}, plan); err != nil {
				t.Fatal(err)
			}
			if pages.Load() != 2 || len(sink.records) != 2 {
				t.Fatal("incremental pagination incomplete")
			}
			for _, rec := range sink.records {
				if len(rec.Key) != 1 || rec.Key[0] != "2026-09-03T00:00:00Z" {
					t.Fatal("watermark did not advance")
				}
			}
		})
	}
}

func TestGitHubOperations(t *testing.T) {
	responses := map[string]string{
		"/orgs/test-org/members":                        `[{"id":1,"login":"alice"}]`,
		"/orgs/test-org/teams":                          `[{"id":21,"name":"Core","slug":"core"}]`,
		"/orgs/test-org/teams/core/members":             `[{"id":1,"login":"alice"}]`,
		"/repos/test-org/one/branches":                  `[{"name":"main","protected":true,"commit":{"sha":"shared"}},{"name":"feature","protected":false,"commit":{"sha":"shared"}}]`,
		"/repos/test-org/one/tags":                      `[{"name":"v1","commit":{"sha":"shared"}}]`,
		"/repos/test-org/one/commits":                   `[{"sha":"shared","commit":{"message":"test","author":{"name":"Alice","date":"2026-09-01T00:00:00Z"}},"author":null,"parents":[]}]`,
		"/repos/test-org/one/contributors":              `[{"id":1,"login":"alice","type":"User","contributions":10}]`,
		"/repos/test-org/one/languages":                 `{"Go":12000,"TypeScript":8000}`,
		"/repos/test-org/one/collaborators":             `[{"id":1,"login":"alice","permissions":{"admin":true},"role_name":"admin"}]`,
		"/repos/test-org/one/actions/workflows":         `{"total_count":1,"workflows":[{"id":30,"name":"CI","path":".github/workflows/ci.yaml","state":"active","created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-02T00:00:00Z"}]}`,
		"/repos/test-org/one/actions/runs":              `{"total_count":1,"workflow_runs":[{"id":31,"workflow_id":30,"run_number":1,"event":"push","status":"completed","conclusion":"success","head_sha":"shared","created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-02T00:00:00Z"}]}`,
		"/repos/test-org/one/actions/runs/31/jobs":      `{"total_count":1,"jobs":[{"id":32,"run_id":31,"name":"test","status":"in_progress","conclusion":null,"completed_at":null,"steps":[{"name":"test","status":"in_progress"}]}]}`,
		"/repos/test-org/one/deployments":               `[{"id":40,"sha":"shared","ref":"main","environment":"production","created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-02T00:00:00Z"}]`,
		"/repos/test-org/one/deployments/40/statuses":   `[{"id":41,"state":"success","created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-02T00:00:00Z"}]`,
		"/repos/test-org/one/traffic/views":             `{"count":5,"uniques":3,"views":[{"timestamp":"2026-09-01T00:00:00Z","count":5,"uniques":3}]}`,
		"/repos/test-org/one/traffic/clones":            `{"count":2,"uniques":1,"clones":[{"timestamp":"2026-09-01T00:00:00Z","count":2,"uniques":1}]}`,
		"/repos/test-org/one/traffic/popular/referrers": `[{"referrer":"example.com","count":3,"uniques":2}]`,
		"/repos/test-org/one/traffic/popular/paths":     `[{"path":"/test-org/one","title":"test","count":3,"uniques":2}]`,
	}
	var commitCalls atomic.Int64
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/orgs/test-org/repos" {
			_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/commits") {
			commitCalls.Add(1)
			if !slices.Contains([]string{"main", "feature"}, r.URL.Query().Get("sha")) || r.URL.Query().Get("since") != "" {
				t.Error("commit walk must enumerate branch history")
			}
		}
		if strings.HasSuffix(r.URL.Path, "/jobs") && r.URL.Query().Get("filter") != "all" {
			t.Error("job walk omitted older attempts")
		}
		if strings.Contains(r.URL.Path, "/traffic/") {
			w.Header().Set("Link", "<http://"+r.Host+"/must-not-follow>; rel=\"next\"")
			if !strings.Contains(r.URL.Path, "/popular/") && r.URL.Query().Get("per") != "day" {
				t.Error("traffic buckets must be daily")
			}
		}
		if payload, ok := responses[r.URL.Path]; ok {
			fmt.Fprint(w, payload)
		} else {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	resources := []string{"branches", "tags", "commits", "contributors", "languages", "members", "teams", "team_members", "collaborators", "workflows", "workflow_runs", "workflow_jobs", "deployments", "deployment_statuses", "traffic_views", "traffic_clones", "traffic_referrers", "traffic_paths"}
	var sink collectSink
	if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: resources}); err != nil {
		t.Fatal(err)
	}
	for _, resource := range resources {
		want := 1
		if resource == "branches" || resource == "commits" {
			want = 2
		}
		if got := len(githubRows(t, &sink, resource)); got != want {
			t.Fatalf("%s rows=%d, want %d", resource, got, want)
		}
	}
	if commitCalls.Load() != 2 {
		t.Fatal("did not traverse both branches")
	}
	commits := githubRows(t, &sink, "commits")
	if commits[0]["branch"] == commits[1]["branch"] || commits[0]["repository_id"] != float64(10) {
		t.Fatal("shared commit lost branch/repository identity")
	}
	if githubRows(t, &sink, "team_members")[0]["team_id"] != float64(21) {
		t.Fatal("team membership lost parent identity")
	}
	if githubRows(t, &sink, "deployment_statuses")[0]["deployment_id"] != float64(40) {
		t.Fatal("deployment status lost parent identity")
	}
	if githubRows(t, &sink, "languages")[0]["languages"].(map[string]any)["Go"] != float64(12000) {
		t.Fatal("language map was not preserved")
	}
}

func TestGitHubCommitsDoNotRequestEmptyRepositories(t *testing.T) {
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orgs/test-org/repos":
			_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "empty")})
		case "/repos/test-org/empty/branches":
			fmt.Fprint(w, `[]`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	var sink collectSink
	if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"commits"}}); err != nil {
		t.Fatal(err)
	}
	if len(sink.records) != 0 {
		t.Fatal("empty repository emitted commits")
	}
}

func TestGitHubProjects(t *testing.T) {
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orgs/test-org/projectsV2":
			fmt.Fprint(w, `[{"id":10,"number":7,"title":"Roadmap","created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-02T00:00:00Z"}]`)
		case "/orgs/test-org/projectsV2/7/fields":
			fmt.Fprint(w, `[{"id":11,"name":"Status","data_type":"single_select","created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-02T00:00:00Z"},{"id":12,"name":"Priority","data_type":"number","created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-02T00:00:00Z"}]`)
		case "/orgs/test-org/projectsV2/7/items":
			q := r.URL.Query()
			id := 21
			if q.Get("after") == "next-cursor" {
				id = 22
			} else {
				q.Set("after", "next-cursor")
				w.Header().Set("Link", "<http://"+r.Host+r.URL.Path+"?"+q.Encode()+">; rel=\"next\"")
			}
			field := q.Get("fields")
			if field == "" {
				field = "1"
			} else if field != "11" && field != "12" {
				t.Errorf("unknown field selection %s", field)
			}
			fmt.Fprintf(w, `[{"id":%d,"content_type":"Issue","content":{"id":31},"created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-02T00:00:00Z","fields":[{"id":%s,"value":"preserved"}]}]`, id, field)
		default:
			t.Errorf("unexpected project request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	var sink collectSink
	if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"projects", "project_fields", "project_items", "project_item_field_values"}}); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int{"projects": 1, "project_fields": 2, "project_items": 2, "project_item_field_values": 4} {
		if got := len(githubRows(t, &sink, name)); got != want {
			t.Fatalf("%s rows=%d want %d", name, got, want)
		}
	}
	keys := map[string]bool{}
	for _, row := range githubRows(t, &sink, "project_item_field_values") {
		if row["project_id"] != float64(10) || row["project_number"] != float64(7) {
			t.Fatal("lost project ancestry")
		}
		fields := row["fields"].([]any)
		if fields[0].(map[string]any)["id"] != row["field_id"] {
			t.Fatal("field selection changed between pages")
		}
		key := fmt.Sprint(row["item_id"], "/", row["field_id"])
		if keys[key] {
			t.Fatal("custom field rows collide")
		}
		keys[key] = true
	}
}

func TestGitHubSecurityAlerts(t *testing.T) {
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/orgs/test-org/repos" {
			_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one"), githubRepository(20, "two")})
			return
		}
		if r.URL.Query().Get("state") != "" {
			t.Error("alert walk must include resolved states")
		}
		if strings.Contains(r.URL.Path, "/secret-scanning/") && r.URL.Query().Get("hide_secret") != "true" {
			t.Error("secret values must be hidden at the API")
		}
		fmt.Fprint(w, `[{"number":1,"state":"resolved","secret_type":"example","created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-02T00:00:00Z","extra":{"preserved":true}}]`)
	})
	var sink collectSink
	resources := []string{"dependabot_alerts", "code_scanning_alerts", "secret_scanning_alerts"}
	if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: resources}); err != nil {
		t.Fatal(err)
	}
	for _, name := range resources {
		rows := githubRows(t, &sink, name)
		if len(rows) != 2 || rows[0]["repository_id"] == rows[1]["repository_id"] || rows[0]["number"] != rows[1]["number"] {
			t.Fatalf("%s lost repository-scoped alert identity", name)
		}
	}
}

func TestGitHubDiscussions(t *testing.T) {
	var commentPages atomic.Int64
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/orgs/test-org/repos" {
			_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
			return
		}
		if r.Method != "POST" || r.URL.Path != "/graphql" {
			t.Errorf("unexpected GraphQL request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		switch {
		case strings.HasPrefix(body.Query, "query Discussions("):
			if body.Variables["owner"] != "test-org" || body.Variables["name"] != "one" {
				t.Error("GraphQL repository scope missing")
			}
			fmt.Fprint(w, `{"data":{"repository":{"discussions":{"nodes":[{"id":"D1","number":7,"title":"Question","body":"Hello","createdAt":"2026-09-01T00:00:00Z","updatedAt":"2026-09-02T00:00:00Z"}],"pageInfo":{"hasNextPage":false,"endCursor":null}}}}}`)
		case strings.HasPrefix(body.Query, "query DiscussionComments("):
			commentPages.Add(1)
			if body.Variables["id"] != "D1" {
				t.Error("discussion node scope missing")
			}
			if body.Variables["after"] == "cursor-1" {
				fmt.Fprint(w, `{"data":{"node":{"comments":{"nodes":[{"id":"C2","body":"Second","createdAt":"2026-09-01T00:00:00Z","updatedAt":"2026-09-02T00:00:00Z"}],"pageInfo":{"hasNextPage":false,"endCursor":null}}}}}`)
			} else {
				fmt.Fprint(w, `{"data":{"node":{"comments":{"nodes":[{"id":"C1","body":"First","createdAt":"2026-09-01T00:00:00Z","updatedAt":"2026-09-02T00:00:00Z"}],"pageInfo":{"hasNextPage":true,"endCursor":"cursor-1"}}}}}`)
			}
		case strings.HasPrefix(body.Query, "query DiscussionReplies("):
			fmt.Fprintf(w, `{"data":{"node":{"replies":{"nodes":[{"id":"R%s","body":"Reply","createdAt":"2026-09-01T00:00:00Z","updatedAt":"2026-09-02T00:00:00Z"}],"pageInfo":{"hasNextPage":false,"endCursor":null}}}}}`, body.Variables["id"])
		default:
			t.Error("unexpected GraphQL query")
		}
	})
	var sink collectSink
	if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"discussions", "discussion_comments", "discussion_replies"}}); err != nil {
		t.Fatal(err)
	}
	if commentPages.Load() != 2 || len(githubRows(t, &sink, "discussion_comments")) != 2 {
		t.Fatal("GraphQL comments were truncated")
	}
	rows := githubRows(t, &sink, "discussion_replies")
	if len(rows) != 2 {
		t.Fatal("replies missing")
	}
	for _, row := range rows {
		if row["repository_id"] != float64(10) || row["discussion_id"] != "D1" || row["discussion_number"] != float64(7) || row["discussion_comment_id"] == nil {
			t.Fatal("four-level ancestry lost")
		}
	}
}

func TestGitHubGraphQLErrorsFailExtraction(t *testing.T) {
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/orgs/test-org/repos" {
			_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
			return
		}
		fmt.Fprint(w, `{"data":{"repository":null},"errors":[{"message":"Resource not accessible by integration"}]}`)
	})
	var sink collectSink
	err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"discussions"}})
	if err == nil || !strings.Contains(err.Error(), "Resource not accessible") {
		t.Fatalf("error=%v, want GraphQL error", err)
	}
}

func TestGitHubStatisticsPollAndPreserveNativeShapes(t *testing.T) {
	for _, tc := range []struct{ name, path, body string }{
		{"commit_activity", "commit_activity", `[{"week":1754784000,"total":7,"days":[1,1,1,1,1,1,1]}]`},
		{"code_frequency", "code_frequency", `[[1754784000,12,-5]]`},
		{"contributor_statistics", "contributors", `[{"author":null,"total":7,"weeks":[{"w":1754784000,"a":12,"d":5,"c":7}]}]`},
		{"participation", "participation", `{"all":[1,2,3],"owner":[1,1,2]}`},
		{"punch_card", "punch_card", `[[0,0,3],[0,1,0]]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int64
			src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/orgs/test-org/repos" {
					_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
					return
				}
				if r.URL.Path != "/repos/test-org/one/stats/"+tc.path {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				if calls.Add(1) == 1 {
					w.WriteHeader(http.StatusAccepted)
					fmt.Fprint(w, `{}`)
					return
				}
				fmt.Fprint(w, tc.body)
			})
			var sink collectSink
			if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{tc.name}}); err != nil {
				t.Fatal(err)
			}
			rows := githubRows(t, &sink, tc.name)
			if calls.Load() != 2 || len(rows) != 1 {
				t.Fatalf("calls=%d rows=%d; pending data must not emit", calls.Load(), len(rows))
			}
			got, _ := json.Marshal(rows[0]["statistics"])
			var want any
			_ = json.Unmarshal([]byte(tc.body), &want)
			wantJSON, _ := json.Marshal(want)
			if string(got) != string(wantJSON) {
				t.Fatalf("statistics shape lost: %s", got)
			}
		})
	}
}

func TestGitHubTimelinePreservesEventsWithoutIDs(t *testing.T) {
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orgs/test-org/repos":
			_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
		case "/repos/test-org/one/issues":
			fmt.Fprint(w, `[{"id":20,"number":7}]`)
		case "/repos/test-org/one/issues/7/timeline":
			fmt.Fprint(w, `[{"id":1,"event":"closed"},{"sha":"abc","event":"committed"},{"event":"cross-referenced","source":{"issue":{"id":21}},"created_at":"2026-09-01T00:00:00Z"},{"event":"line-commented","comments":[{"id":3}]}]`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	var sink collectSink
	if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"issue_timeline"}}); err != nil {
		t.Fatal(err)
	}
	rows := githubRows(t, &sink, "issue_timeline")
	if len(rows) != 4 {
		t.Fatal("timeline dropped an event shape")
	}
	for _, row := range rows {
		if row["issue_id"] != float64(20) || row["repository_id"] != float64(10) {
			t.Fatal("timeline lost issue ancestry")
		}
	}
	schema, err := src.Schema(context.Background(), "issue_timeline")
	if err != nil || len(schema.PrimaryKey) != 0 {
		t.Fatal("timeline must not advertise an invented upsert key")
	}
}

func TestGitHubAnonymousContributors(t *testing.T) {
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/orgs/test-org/repos" {
			_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
			return
		}
		if r.URL.Query().Get("anon") != "true" {
			t.Error("anonymous contributors were not requested")
		}
		if r.URL.Query().Get("page") != "2" {
			w.Header().Set("Link", "<http://"+r.Host+r.URL.Path+"?anon=true&page=2&per_page=100>; rel=\"next\"")
			fmt.Fprint(w, `[{"id":1,"login":"alice","type":"User","contributions":3}]`)
		} else {
			fmt.Fprint(w, `[{"name":"Unknown","email":"author@example.com","type":"Anonymous","contributions":2}]`)
		}
	})
	var sink collectSink
	if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"anonymous_contributors"}}); err != nil {
		t.Fatal(err)
	}
	rows := githubRows(t, &sink, "anonymous_contributors")
	if len(rows) != 1 || rows[0]["email"] != "author@example.com" {
		t.Fatal("anonymous contributor identity lost after filtered empty page")
	}
}

func TestGitHubPullRequestFilesAndEvents(t *testing.T) {
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orgs/test-org/repos":
			_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
		case "/repos/test-org/one/pulls":
			fmt.Fprint(w, `[{"id":20,"number":7}]`)
		case "/repos/test-org/one/pulls/7/files":
			fmt.Fprint(w, `[{"filename":"new.go","status":"renamed","previous_filename":"old.go","patch":null}]`)
		case "/repos/test-org/one/events":
			fmt.Fprint(w, `[{"id":"1234567890123456789","type":"PushEvent","created_at":"2026-09-01T00:00:00Z","payload":{"ref":"refs/heads/main"}}]`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	var sink collectSink
	if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"pull_request_files", "repository_events"}}); err != nil {
		t.Fatal(err)
	}
	if githubRows(t, &sink, "pull_request_files")[0]["pull_request_id"] != float64(20) {
		t.Fatal("file lost PR ancestry")
	}
	if githubRows(t, &sink, "repository_events")[0]["id"] != "1234567890123456789" {
		t.Fatal("event ID lost precision")
	}
}

func TestGitHubNullableResponseFields(t *testing.T) {
	for _, tc := range []struct {
		resource, suffix, body string
		nullFields             []string
	}{
		{"workflow_runs", "/actions/runs", `{"workflow_runs":[{"id":1,"workflow_id":2,"run_number":1,"head_sha":"abc","event":"push","status":null,"created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-01T00:00:00Z"}]}`, []string{"status"}},
		{"code_scanning_alerts", "/code-scanning/alerts", `[{"number":1,"state":null,"created_at":"2026-09-01T00:00:00Z"}]`, []string{"state", "updated_at"}},
		{"secret_scanning_alerts", "/secret-scanning/alerts", `[{"number":1}]`, []string{"state", "created_at", "secret_type"}},
		{"repository_events", "/events", `[{"id":"1","type":null,"created_at":null}]`, []string{"type", "created_at"}},
	} {
		t.Run(tc.resource, func(t *testing.T) {
			src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/orgs/test-org/repos" {
					_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
				} else if r.URL.Path == "/repos/test-org/one"+tc.suffix {
					fmt.Fprint(w, tc.body)
				} else {
					t.Errorf("unexpected path %s", r.URL.Path)
					http.NotFound(w, r)
				}
			})
			var sink collectSink
			if err := src.Extract(t.Context(), &sink, filament.ExtractOpts{Resources: []string{tc.resource}}); err != nil {
				t.Fatal(err)
			}
			rows := githubRows(t, &sink, tc.resource)
			if len(rows) != 1 {
				t.Fatalf("rows=%d, want 1", len(rows))
			}
			for _, field := range tc.nullFields {
				if rows[0][field] != nil {
					t.Errorf("%s=%v, want null", field, rows[0][field])
				}
			}
		})
	}
}

func TestGitHubTeamsPermissionAndResourceSelection(t *testing.T) {
	for _, selection := range []string{"repository resources", "teams selected", "team members selected"} {
		t.Run(selection, func(t *testing.T) {
			var teamCalls, childCalls atomic.Int64
			src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/orgs/test-org/repos":
					_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
				case "/orgs/test-org/members":
					fmt.Fprint(w, `[{"id":1,"login":"alice"}]`)
				case "/orgs/test-org/teams":
					teamCalls.Add(1)
					w.WriteHeader(http.StatusForbidden)
					fmt.Fprint(w, `{"message":"Resource not accessible by personal access token","documentation_url":"https://docs.github.com/rest/teams/teams#list-teams","status":"403"}`)
				case "/repos/test-org/one/stargazers/count":
					childCalls.Add(1)
					fmt.Fprint(w, `{"count":2}`)
				case "/repos/test-org/one/forks":
					childCalls.Add(1)
					fmt.Fprint(w, `[{"id":100,"name":"fork","full_name":"alice/fork"}]`)
				case "/repos/test-org/one/issues/comments":
					childCalls.Add(1)
					fmt.Fprint(w, `[{"id":200,"issue_url":"https://api.github.com/repos/test-org/one/issues/1","body":"comment","created_at":"2026-09-01T00:00:00Z","updated_at":"2026-09-01T00:00:00Z"}]`)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					http.NotFound(w, r)
				}
			})
			resources := []string{"repositories", "members", "star_count", "forks", "issue_comments"}
			switch selection {
			case "teams selected":
				resources = append(resources, "teams")
			case "team members selected":
				resources = append(resources, "team_members")
			}
			var sink collectSink
			err := src.Extract(t.Context(), &sink, filament.ExtractOpts{Resources: resources})
			if selection != "repository resources" {
				if err == nil || !strings.Contains(err.Error(), "before child resources (2/3 top-level resources succeeded)") || !strings.Contains(err.Error(), "teams") || !strings.Contains(err.Error(), "HTTP 403") {
					t.Fatalf("expected an explicit top-level teams failure, got %v", err)
				}
				if teamCalls.Load() != 1 || childCalls.Load() != 0 {
					t.Fatalf("team calls=%d, child calls=%d; permission denial must be fatal without retry", teamCalls.Load(), childCalls.Load())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if teamCalls.Load() != 0 || childCalls.Load() != 3 {
				t.Fatalf("team calls=%d, child calls=%d; unrelated selections must not fetch teams", teamCalls.Load(), childCalls.Load())
			}
			for _, name := range resources {
				if got := len(githubRows(t, &sink, name)); got != 1 {
					t.Errorf("%s rows=%d, want 1", name, got)
				}
			}
		})
	}
}

func TestGitHubManifestRateLimits(t *testing.T) {
	m, err := manifest.Parse(githubManifest)
	if err != nil {
		t.Fatal(err)
	}
	spec := m.Connection.RateLimit
	body := []byte(`{"message":"You have exceeded a secondary rate limit.","documentation_url":"https://docs.github.com/rest/overview/resources-in-the-rest-api#secondary-rate-limits"}`)
	for _, status := range []int{403, 429} {
		resp := &http.Response{StatusCode: status, Header: make(http.Header)}
		if !isRateLimited(resp, body, spec) {
			t.Fatalf("HTTP %d was not recognized", status)
		}
		if got := rateLimitDelay(resp, body, 0, spec); got != time.Minute {
			t.Fatalf("delay=%s, want one minute", got)
		}
		if got := rateLimitDelay(resp, body, 2, spec); got != 4*time.Minute {
			t.Fatalf("delay=%s, want four minutes", got)
		}
		resp.Header.Set("Retry-After", "2")
		if got := rateLimitDelay(resp, body, 0, spec); got != 2*time.Second {
			t.Fatalf("explicit delay=%s", got)
		}
	}
	resp := &http.Response{StatusCode: 403, Header: make(http.Header)}
	if isRateLimited(resp, []byte(`{"message":"Resource not accessible by personal access token"}`), spec) {
		t.Fatal("permission denial was classified as throttling")
	}
	resp.Header.Set("X-RateLimit-Remaining", "0")
	resp.Header.Set("X-RateLimit-Reset", fmt.Sprint(time.Now().Add(5*time.Minute).Unix()))
	if !isRateLimited(resp, nil, spec) {
		t.Fatal("exhausted budget was not recognized")
	}
	if got := rateLimitDelay(resp, nil, 0, spec); got < 299*time.Second || got > 300*time.Second {
		t.Fatalf("budget reset delay=%s", got)
	}
	resp.Header.Del("X-RateLimit-Remaining")
	resp.Header.Set("Retry-After", "120")
	if !isRateLimited(resp, nil, spec) {
		t.Fatal("retry header was not recognized")
	}
	if got := rateLimitDelay(resp, nil, 0, spec); got != 120*time.Second {
		t.Fatalf("retry header delay=%s", got)
	}
}
