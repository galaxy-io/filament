package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/pagination"
	"github.com/galaxy-io/filament/connectors/internal/ndjson"
	"github.com/galaxy-io/filament/rowmodel"
)

func TestSourceExtractFromSeedsPaginationCursor(t *testing.T) {
	ctx := context.Background()
	var gotCursor string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCursor = r.URL.Query().Get("start_cursor")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"id":"b","name":"second page"}],"has_more":false}`)
	}))
	defer api.Close()

	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"manifest_path": writeTestManifest(t, api.URL),
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	schema, err := src.Schema(ctx, "items")
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	if len(schema.PrimaryKey) != 1 || schema.PrimaryKey[0] != "id" {
		t.Fatalf("primary key = %v, want [id]", schema.PrimaryKey)
	}

	prev := map[string]filament.Checkpoint{
		"items": checkpoint.KeysetCheckpoint{
			Cols:   []string{"cursor"},
			Types:  []string{"string"},
			Shards: []checkpoint.KeysetShard{{Key: []string{"two"}}},
		}.ToCheckpoint("items"),
	}
	var sink collectSink
	if err := src.ExtractFrom(ctx, &sink, filament.ExtractOpts{Resources: []string{"items"}, Parallelism: 1}, prev); err != nil {
		t.Fatalf("extract from: %v", err)
	}
	if gotCursor != "two" {
		t.Fatalf("start_cursor = %q, want %q", gotCursor, "two")
	}
	if len(sink.records) != 1 {
		t.Fatalf("records = %d, want 1", len(sink.records))
	}
	if sink.records[0].ID != "b" {
		t.Fatalf("record id = %q, want b", sink.records[0].ID)
	}
	if len(sink.records[0].Key) != 1 {
		t.Fatalf("record key = %v, want one pagination checkpoint", sink.records[0].Key)
	}
	state, err := pagination.ResumeFrom(sink.records[0].Key[0])
	if err != nil || state.Cursor != "two" {
		t.Fatalf("record checkpoint = %v (%v), want cursor two", state, err)
	}
}

func TestSourceTestConnectionMakesOneAuthenticatedRequest(t *testing.T) {
	var calls int
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/probe" {
			t.Errorf("request = %s %s, want GET /probe", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer good-token" {
			fmt.Fprint(w, `{"error":"invalid token"}`)
			return
		}
		fmt.Fprint(w, `{"items":[]}`)
	}))
	defer api.Close()

	data := []byte(fmt.Sprintf(`
version: 1
name: probe
display_name: Probe
description: Test probe connector.
dark_logo_url: https://cdn.example.com/probe-dark.svg
light_logo_url: https://cdn.example.com/probe-light.svg
config:
  token: {type: secret, required: true}
connection:
  base_url: %s
  auth:
    bearer: config.token
resources:
  - name: probe
    path: /probe
    response:
      records: $.items
      error:
        path: error
        when_present: true
`, api.URL))
	src := NewManifest(data)
	if err := src.TestConnection(context.Background(), filament.NewConfig(map[string]any{
		"token": "good-token",
	})); err != nil {
		t.Fatalf("test connection: %v", err)
	}
	if calls != 1 {
		t.Fatalf("requests = %d, want exactly 1", calls)
	}

	err := src.TestConnection(context.Background(), filament.NewConfig(map[string]any{
		"token": "bad-token",
	}))
	if err == nil || !strings.Contains(err.Error(), "invalid token") {
		t.Fatalf("bad credentials error = %v, want wrapped API error", err)
	}
}

func TestConnectorPopulatesEnvironmentTemplateScope(t *testing.T) {
	t.Setenv("FILAMENT_HTTP_BASE_URL", "https://api.example.com")
	t.Setenv("FILAMENT_HTTP_TOKEN", "secret=value")
	t.Setenv("FILAMENT_HTTP_PATH", "environment-items")
	t.Setenv("FILAMENT_HTTP_HEADER", "from-environment")

	m, err := manifest.Parse([]byte(`
version: 1
name: environment
display_name: Environment
description: Environment template scope test.
dark_logo_url: https://cdn.example.com/environment-dark.svg
light_logo_url: https://cdn.example.com/environment-light.svg
connection:
  base_url: "{{ env.FILAMENT_HTTP_BASE_URL }}"
  auth:
    bearer: "{{ env.FILAMENT_HTTP_TOKEN }}"
  headers:
    X-Environment: "{{ env.FILAMENT_HTTP_HEADER }}"
resources:
  - name: items
    path: "/{{ env.FILAMENT_HTTP_PATH }}"
    response:
      records: $.items
`))
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	c := &Connector{}
	c.SetManifest(m)
	if err := c.Configure(context.Background()); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer c.Teardown(context.Background())

	req, err := c.builder.Build(context.Background(), m.Resources[0], c.scopeFor(nil, "", nil))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if got, want := req.URL.String(), "https://api.example.com/environment-items"; got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
	if got, want := req.Header.Get("Authorization"), "Bearer secret=value"; got != want {
		t.Fatalf("Authorization = %q, want %q", got, want)
	}
	if got, want := req.Header.Get("X-Environment"), "from-environment"; got != want {
		t.Fatalf("X-Environment = %q, want %q", got, want)
	}
}

func TestSourcePlanResumeExpandsManifestResources(t *testing.T) {
	ctx := context.Background()
	api := httptest.NewServer(http.NotFoundHandler())
	defer api.Close()

	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"manifest_path": writeTestManifest(t, api.URL),
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	plan, err := src.PlanResume(ctx, nil, nil)
	if err != nil {
		t.Fatalf("plan resume: %v", err)
	}
	cp := plan["items"]
	if cp == nil {
		t.Fatalf("missing items checkpoint in plan: %#v", plan)
	}
	ks, ok := checkpoint.ParseKeyset(cp)
	if !ok {
		t.Fatalf("checkpoint did not parse as keyset: %#v", cp.Raw())
	}
	if len(ks.Shards) != 1 {
		t.Fatalf("shards = %d, want 1", len(ks.Shards))
	}
}

func TestSourceFullResumeDoesNotApplyIncrementalWatermark(t *testing.T) {
	ctx := context.Background()
	var gotSince string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSince = r.URL.Query().Get("since")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"id":"c","updated_at":"2026-01-02T00:00:00Z"}],"has_more":false}`)
	}))
	defer api.Close()

	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"manifest_path": writeIncrementalTestManifest(t, api.URL),
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	plan, err := src.PlanResume(ctx, []string{"items"}, nil)
	if err != nil {
		t.Fatalf("plan resume: %v", err)
	}
	ks, ok := checkpoint.ParseKeyset(plan["items"])
	if !ok {
		t.Fatal("plan did not parse as keyset")
	}
	if got, want := ks.Cols, []string{"pagination_state"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("checkpoint cols = %v, want %v", got, want)
	}

	prev := map[string]filament.Checkpoint{
		"items": checkpoint.KeysetCheckpoint{
			Cols:   []string{"cursor", "items_since"},
			Types:  []string{"string", "string"},
			Shards: []checkpoint.KeysetShard{{Key: []string{"", "2026-01-01T00:00:00Z"}}},
		}.ToCheckpoint("items"),
	}
	var sink collectSink
	if err := src.ExtractFrom(ctx, &sink, filament.ExtractOpts{Resources: []string{"items"}, Parallelism: 1}, prev); err != nil {
		t.Fatalf("extract from: %v", err)
	}
	if gotSince != "" {
		t.Fatalf("since = %q, want no watermark on a full route", gotSince)
	}
	if len(sink.records) != 1 {
		t.Fatalf("records = %d, want 1", len(sink.records))
	}
	if len(sink.records[0].Key) != 1 {
		t.Fatalf("record key = %v, want a pagination checkpoint and no incremental watermark", sink.records[0].Key)
	}
}

func TestSourcePlanIncrementalUsesOnlyDurableWatermark(t *testing.T) {
	ctx := context.Background()
	api := httptest.NewServer(http.NotFoundHandler())
	defer api.Close()

	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"manifest_path": writeIncrementalTestManifest(t, api.URL),
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	current := checkpoint.KeysetCheckpoint{
		Mode:   checkpoint.ModeIncremental,
		Cols:   []string{"items_since"},
		Types:  []string{"timestamptz"},
		Shards: []checkpoint.KeysetShard{{Key: []string{"2026-01-01T00:00:00Z"}}},
	}.ToCheckpoint("items")
	plan, err := src.PlanIncremental(ctx, []string{"items"}, map[string]filament.Checkpoint{"items": current}, map[string]filament.ResourceCursorConfig{
		"items": {Field: "updated_at", LookbackSeconds: 60},
	})
	if err != nil {
		t.Fatalf("plan incremental: %v", err)
	}
	ks, ok := checkpoint.ParseKeyset(plan["items"])
	if !ok || ks.Mode != checkpoint.ModeIncremental {
		t.Fatalf("checkpoint = %#v, want incremental", plan["items"].Raw())
	}
	if len(ks.Cols) != 1 || ks.Cols[0] != "items_since" || len(ks.Shards) != 1 || len(ks.Shards[0].Key) != 1 || ks.Shards[0].Key[0] != "2026-01-01T00:00:00Z" {
		t.Fatalf("incremental checkpoint = %#v, want watermark without page cursor", ks)
	}
	columns, err := src.CursorColumns(ctx, "items")
	if err != nil {
		t.Fatal(err)
	}
	if len(columns) != 1 || columns[0].Name != "updated_at" || !columns[0].Recommended || columns[0].Configurable || !columns[0].SupportsLookback {
		t.Fatalf("cursor columns = %#v", columns)
	}
	if err := filament.ValidateSourceIngestion(src.Spec(), filament.IngestionIncrementalUpsert); err != nil {
		t.Fatalf("incremental policy not advertised: %v", err)
	}
}

func TestSourceIncrementalExtractionAppliesRouteLookbackAndDropsPageCursor(t *testing.T) {
	ctx := context.Background()
	var gotSince string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSince = r.URL.Query().Get("since")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"id":"c","updated_at":"2026-01-02T00:00:00Z"}],"next_cursor":"page-2","has_more":false}`)
	}))
	defer api.Close()

	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"manifest_path": writeIncrementalTestManifest(t, api.URL),
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)
	prev := map[string]filament.Checkpoint{
		"items": checkpoint.KeysetCheckpoint{
			Mode: checkpoint.ModeIncremental, Cols: []string{"items_since"}, Types: []string{"timestamptz"},
			Shards: []checkpoint.KeysetShard{{Key: []string{"2026-01-01T00:00:00Z"}}},
		}.ToCheckpoint("items"),
	}
	plan, err := src.PlanIncremental(ctx, []string{"items"}, prev, map[string]filament.ResourceCursorConfig{
		"items": {Field: "updated_at", LookbackSeconds: 60},
	})
	if err != nil {
		t.Fatal(err)
	}
	var sink collectSink
	progress := make(chan filament.SourceProgress, 4)
	if err := src.ExtractFrom(ctx, &sink, filament.ExtractOpts{
		Resources:   []string{"items"},
		Parallelism: 1,
		Observe:     func(p filament.SourceProgress) { progress <- p },
	}, plan); err != nil {
		t.Fatalf("extract incremental: %v", err)
	}
	close(progress)
	if gotSince != "2025-12-31T23:59:00Z" {
		t.Fatalf("since = %q, want route lookback applied", gotSince)
	}
	if len(sink.records) != 1 || len(sink.records[0].Key) != 1 || sink.records[0].Key[0] != "2026-01-02T00:00:00Z" {
		t.Fatalf("record checkpoint key = %#v, want only durable watermark", sink.records)
	}
	var sawPage, sawWatermark bool
	for event := range progress {
		switch event.Kind {
		case filament.SourceProgressPageFetched:
			sawPage = event.Resource == "items" && event.Records == 1 && event.Bytes > 0
		case filament.SourceProgressWatermarkAdvanced:
			sawWatermark = event.Resource == "items" && event.Checkpoint != nil && event.Checkpoint.String("items_since") == "2026-01-02T00:00:00Z"
		}
	}
	if !sawPage || !sawWatermark {
		t.Fatalf("progress page=%t watermark=%t", sawPage, sawWatermark)
	}
}

func TestIncrementalRecordReducerNeverRegressesFanoutWatermark(t *testing.T) {
	src := &Source{incrementalResources: map[string]manifest.IncrementalSpec{
		"messages": {CursorField: "ts", CheckpointKey: "messages_since", Comparator: "numeric"},
	}}
	reducer := newIncrementalRecordReducer(src, map[string]map[string]string{
		"messages": {"messages_since": "10"},
	})
	for i, value := range []string{"30", "20"} {
		got, err := reducer.record(record{
			Resource: "messages",
			ID:       "x",
			Data:     []byte(`{"id":"x"}`),
			Key:      []string{value},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Key) != 1 || got.Key[0] != "30" {
			t.Fatalf("record %d key = %v, want monotonic [30]", i, got.Key)
		}
	}
}

func TestSourcePlanResourcesExpandsSelectedNotionDatabase(t *testing.T) {
	ctx := context.Background()
	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"manifest_path": writeNotionTestManifest(t, "https://notion.test"),
		"api_key":       "test-token",
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	got, err := src.PlanResources(ctx, []string{"db1"}, []string{encodeSelector("database", "db1")})
	if err != nil {
		t.Fatalf("plan resources: %v", err)
	}
	want := []string{"databases", "pages_db1"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("resources = %v, want %v", got, want)
	}
	schema, err := src.Schema(ctx, "pages_db1")
	if err != nil {
		t.Fatalf("schema pages_db1: %v", err)
	}
	if schema.Resource != "pages_db1" || len(schema.PrimaryKey) != 1 || schema.PrimaryKey[0] != "id" {
		t.Fatalf("schema = %#v, want dynamic pages schema with id primary key", schema)
	}
	fields := map[string]filament.LogicalType{}
	for _, field := range schema.Fields {
		fields[field.Name] = field.Logical
	}
	if fields["database_id"] != filament.LogicalString || fields["last_edited_time"] != filament.LogicalTimestampTZ || fields["properties"] != filament.LogicalJSON {
		t.Fatalf("schema fields = %#v, want typed notion page fields", fields)
	}
}

func TestSourcePlanResourcesKeepsSingleStaticResource(t *testing.T) {
	ctx := context.Background()
	src := NewAttio()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"api_key": "test-token",
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	got, err := src.PlanResources(ctx, []string{"objects"}, []string{"objects"})
	if err != nil {
		t.Fatalf("plan resources: %v", err)
	}
	if len(got) != 1 || got[0] != "objects" {
		t.Fatalf("resources = %v, want [objects]", got)
	}
}

func TestNewHTTPRecordWrapsUnprojectedPayload(t *testing.T) {
	rec := newHTTPRecord(
		"databases",
		[]byte(`{"id":"db1"}`),
		[]byte(`{"id":"db1","title":[{"plain_text":"Team Tasks"}]}`),
		false,
	)
	var got map[string]json.RawMessage
	if err := json.Unmarshal(rec.Data, &got); err != nil {
		t.Fatalf("record data is not json: %v", err)
	}
	if string(got["id"]) != `"db1"` || rec.ID != "db1" {
		t.Fatalf("id = %s / %q, want db1", got["id"], rec.ID)
	}
	var payload map[string]any
	if err := json.Unmarshal(got["data"], &payload); err != nil {
		t.Fatalf("data payload is not json: %v", err)
	}
	if payload["id"] != "db1" {
		t.Fatalf("payload id = %v, want db1", payload["id"])
	}
}

func TestSourceNotionHTTPAPIManifestDiscoverAndExtractSelectedDatabase(t *testing.T) {
	ctx := context.Background()
	var sawAuth, sawVersion bool
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer test-token" {
			sawAuth = true
		}
		if r.Header.Get("Notion-Version") == "2022-06-28" {
			sawVersion = true
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/search":
			if r.Method != http.MethodPost {
				t.Fatalf("search method = %s, want POST", r.Method)
			}
			fmt.Fprint(w, `{
				"object":"list",
				"results":[
					{"object":"database","id":"db1","title":[{"plain_text":"Team Tasks"}],"last_edited_time":"2026-01-01T00:00:00Z"},
					{"object":"database","id":"db2","title":[{"plain_text":"Archive"}],"last_edited_time":"2026-01-01T00:00:00Z"}
				],
				"has_more":false
			}`)
		case "/v1/databases/db1/query":
			if r.Method != http.MethodPost {
				t.Fatalf("query method = %s, want POST", r.Method)
			}
			fmt.Fprint(w, `{
				"object":"list",
				"results":[
					{"object":"page","id":"page1","created_time":"2026-01-02T00:00:00Z","last_edited_time":"2026-01-03T00:00:00Z","properties":{"Name":{"type":"title","title":[{"plain_text":"Ship port"}]}}}
				],
				"has_more":false
			}`)
		case "/v1/databases/db2/query":
			t.Fatal("db2 should have been filtered before page fan-out")
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"manifest_path": writeNotionTestManifest(t, api.URL),
		"api_key":       "test-token",
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	discovered, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	var dbSelector string
	for _, resource := range discovered.Resources {
		if resource.Name == "db1" {
			dbSelector = resource.Selector
			if resource.DisplayName != "Team Tasks" {
				t.Fatalf("display name = %q, want Team Tasks", resource.DisplayName)
			}
		}
	}
	if dbSelector == "" {
		t.Fatalf("db1 selector not found in discovery: %#v", discovered.Resources)
	}

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Selectors: []string{dbSelector}, Parallelism: 1}); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !sawAuth {
		t.Fatal("Notion bearer auth header was not applied")
	}
	if !sawVersion {
		t.Fatal("Notion-Version header was not applied")
	}

	got := map[string]map[string]bool{}
	for _, rec := range sink.records {
		var data map[string]any
		if err := json.Unmarshal(rec.Data, &data); err != nil {
			t.Fatalf("record %s invalid json: %v", rec.ID, err)
		}
		if rec.Resource == "pages_db1" && rec.ID == "page1" {
			if data["database_id"] != "db1" {
				t.Fatalf("page database_id = %v, want db1", data["database_id"])
			}
			if data["last_edited_time"] != "2026-01-03T00:00:00Z" {
				t.Fatalf("page last_edited_time = %v", data["last_edited_time"])
			}
			if _, ok := data["properties"].(map[string]any); !ok {
				t.Fatalf("page properties = %#v, want object", data["properties"])
			}
			if _, ok := data["raw"].(map[string]any); !ok {
				t.Fatalf("page raw = %#v, want object", data["raw"])
			}
			if _, ok := data["data"]; ok {
				t.Fatalf("page still has generic data envelope: %#v", data)
			}
		}
		if got[rec.Resource] == nil {
			got[rec.Resource] = map[string]bool{}
		}
		got[rec.Resource][rec.ID] = true
	}
	if !got["databases"]["db1"] {
		t.Fatalf("selected database db1 not emitted: %#v", got)
	}
	if got["databases"]["db2"] {
		t.Fatalf("unselected database db2 emitted: %#v", got)
	}
	if !got["pages_db1"]["page1"] {
		t.Fatalf("page from selected database not emitted: %#v", got)
	}
}

func TestNewNotionSpecHidesManifestPath(t *testing.T) {
	ctx := context.Background()
	src := NewNotion()
	spec := src.Spec()
	if spec.Name != "notion" {
		t.Fatalf("name = %q, want notion", spec.Name)
	}
	if spec.DisplayName != "Notion" {
		t.Fatalf("display name = %q, want Notion", spec.DisplayName)
	}
	if spec.Description == "" || spec.DarkLogoURL != "https://cdn.getgalaxy.io/sources/source-icon-notion-dark.svg" || spec.LightLogoURL != "https://cdn.getgalaxy.io/sources/source-icon-notion-light.svg" {
		t.Fatalf("catalog metadata = %#v, want Notion description and logo URLs", spec)
	}
	if len(spec.Config.Fields) != 1 || spec.Config.Fields[0].Name != "api_key" || spec.Config.Fields[0].Type != filament.FieldSecret {
		t.Fatalf("config fields = %#v, want api_key secret only", spec.Config.Fields)
	}
	if err := src.Validate(filament.NewConfig(map[string]any{})); err == nil {
		t.Fatal("validate without api_key succeeded")
	}
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "test-token"})); err != nil {
		t.Fatalf("configure embedded notion manifest: %v", err)
	}
	defer src.Teardown(ctx)
	discovered, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		t.Fatalf("discover static Notion resources: %v", err)
	}
	want := []string{"users", "databases", "pages", "blocks"}
	if len(discovered.Resources) != len(want) {
		t.Fatalf("resources = %#v, want %v", discovered.Resources, want)
	}
	for i, name := range want {
		if discovered.Resources[i].Name != name || discovered.Resources[i].Selector != name {
			t.Fatalf("resource[%d] = %#v, want stable %q selector", i, discovered.Resources[i], name)
		}
	}
}

func TestEmbeddedCatalogMetadata(t *testing.T) {
	tests := []struct {
		name, description, darkLogo, lightLogo string
		source                                 *Source
	}{
		{
			name: "github", source: NewGitHub(),
			description: "Code hosting platform for version control, collaboration, and software development workflows.",
			darkLogo:    "https://cdn.getgalaxy.io/sources/source-icon-github-dark.svg",
			lightLogo:   "https://cdn.getgalaxy.io/sources/source-icon-github-light.svg",
		},
		{
			name: "slack", source: NewSlack(),
			description: "Messaging and collaboration platform designed for teams to communicate and work together efficiently.",
			darkLogo:    "https://cdn.getgalaxy.io/sources/source-icon-slack-dark.svg",
			lightLogo:   "https://cdn.getgalaxy.io/sources/source-icon-slack-light.svg",
		},
		{
			name: "attio", source: NewAttio(),
			description: "CRM platform designed for modern teams to centralize customer data, pipelines, and workflows.",
			darkLogo:    "https://cdn.getgalaxy.io/sources/source-icon-attio-dark.svg",
			lightLogo:   "https://cdn.getgalaxy.io/sources/source-icon-attio-light.svg",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := test.source.Spec()
			if spec.Description != test.description ||
				spec.DarkLogoURL != test.darkLogo ||
				spec.LightLogoURL != test.lightLogo {
				t.Fatalf("metadata = %#v", spec)
			}
		})
	}
}

func TestNewAttioSpecAndEmbeddedManifest(t *testing.T) {
	ctx := context.Background()
	src := NewAttio()
	spec := src.Spec()
	if spec.Name != "attio" || spec.DisplayName != "Attio" {
		t.Fatalf("spec identity = %q/%q, want attio/Attio", spec.Name, spec.DisplayName)
	}
	if len(spec.Config.Fields) != 1 {
		t.Fatalf("config fields = %#v, want api_key", spec.Config.Fields)
	}
	field := spec.Config.Fields[0]
	if field.Name != "api_key" || field.Type != filament.FieldSecret || !field.Required {
		t.Fatalf("api_key field = %#v, want required secret", field)
	}
	if err := src.Validate(filament.NewConfig(map[string]any{})); err == nil {
		t.Fatal("validate without API key succeeded")
	}
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"api_key": "test-api-key",
	})); err != nil {
		t.Fatalf("configure embedded Attio manifest: %v", err)
	}
	defer src.Teardown(ctx)

	discovered, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	want := []string{
		"objects", "records", "object_attributes", "object_views", "record_entries", "files",
		"lists", "entries", "list_attributes", "list_views", "workspace_members", "notes",
		"tasks", "threads", "meetings", "call_recordings", "transcripts", "webhooks",
		"people", "companies", "deals", "users", "workspaces",
	}
	if len(discovered.Resources) != len(want) {
		t.Fatalf("resources = %#v, want %v", discovered.Resources, want)
	}
	for i, name := range want {
		if discovered.Resources[i].Name != name {
			t.Fatalf("resource[%d] = %q, want %q", i, discovered.Resources[i].Name, name)
		}
	}
}

func TestAttioUsesAPIKeyAsBearerToken(t *testing.T) {
	ctx := context.Background()
	var authorization atomic.Value // resources are fetched concurrently
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization.Store(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/objects":
			fmt.Fprint(w, `{"data":[{"id":{"workspace_id":"14beef7a-99f7-4534-a87e-70b564330a4c","object_id":"97052eb9-e65e-443f-a297-f2d9a4a7f795"},"api_slug":"people","singular_noun":"Person","plural_noun":"People","created_at":"2022-11-21T13:22:49Z"}]}`)
		case "/v2/lists":
			fmt.Fprint(w, `{"data":[{"id":{"workspace_id":"14beef7a-99f7-4534-a87e-70b564330a4c","list_id":"33ebdbe9-e529-47c9-b894-0ba25e9c15c0"},"api_slug":"sales","name":"Sales","parent_object":["companies"],"workspace_access":null,"workspace_member_access":[],"created_by_actor":null,"created_at":"2022-11-21T13:22:49Z"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(attioManifest), "https://api.attio.com", api.URL, 1))
	src := NewManifest(manifestData)
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "attio-key"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"objects", "lists"}}); err != nil {
		t.Fatalf("extract Attio catalog: %v", err)
	}
	if got, _ := authorization.Load().(string); got != "Bearer attio-key" {
		t.Fatalf("Authorization = %q, want Bearer attio-key", got)
	}
	if len(sink.records) != 2 {
		t.Fatalf("records = %#v, want one Attio object and list", sink.records)
	}
}

func TestSlackEmbeddedManifestAndMessageFanOut(t *testing.T) {
	ctx := context.Background()
	schemaFields := map[string]filament.ConfigField{}
	for _, field := range NewSlack().Spec().Config.Fields {
		schemaFields[field.Name] = field
	}
	conversationTypesField := schemaFields["conversation_types"]
	if conversationTypesField.Type != filament.FieldList || len(conversationTypesField.Enum) != 4 {
		t.Fatalf("conversation_types field = %#v, want four-option list", conversationTypesField)
	}
	var authorization string
	var historyChannels []string
	var replyScopes []string
	var conversationTypes string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users.conversations":
			conversationTypes = r.URL.Query().Get("types")
			fmt.Fprint(w, `{"ok":true,"channels":[{"id":"C123","name":"general","is_channel":true}],"response_metadata":{"next_cursor":""}}`)
		case "/conversations.history":
			historyChannels = append(historyChannels, r.URL.Query().Get("channel"))
			if r.URL.Query().Get("cursor") == "" {
				fmt.Fprint(w, `{"ok":true,"messages":[{"type":"message","user":"U123","text":"hello","ts":"1710000000.000001"}],"response_metadata":{"next_cursor":"next-page"}}`)
				return
			}
			fmt.Fprint(w, `{"ok":true,"messages":[{"type":"message","user":"U456","text":"world","ts":"1710000001.000002"}],"response_metadata":{"next_cursor":""}}`)
		case "/conversations.replies":
			replyScopes = append(replyScopes, r.URL.Query().Get("channel")+"|"+r.URL.Query().Get("ts"))
			fmt.Fprintf(w, `{"ok":true,"messages":[{"type":"message","user":"U789","text":"reply","ts":"%s"}],"response_metadata":{"next_cursor":""}}`, r.URL.Query().Get("ts"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(slackManifest), "https://slack.com/api", api.URL, 1))
	src := NewManifest(manifestData)
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"token": "xoxb-test"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)
	columns, err := src.CursorColumns(ctx, "messages")
	if err != nil {
		t.Fatalf("message cursor columns: %v", err)
	}
	if len(columns) != 1 || columns[0].Name != "ts" || !columns[0].Recommended || !columns[0].SupportsLookback || !strings.Contains(columns[0].Warning, "Fan-out") {
		t.Fatalf("message cursor columns = %#v", columns)
	}

	discovered, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	wantResources := []string{"team", "users", "user_groups", "conversations", "messages", "thread_replies", "files", "bookmarks", "pins"}
	if len(discovered.Resources) != len(wantResources) {
		t.Fatalf("resources = %#v, want %v", discovered.Resources, wantResources)
	}
	for i, name := range wantResources {
		if discovered.Resources[i].Name != name || discovered.Resources[i].Selector != name {
			t.Fatalf("resource[%d] = %#v, want %q", i, discovered.Resources[i], name)
		}
	}

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"thread_replies"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract thread replies: %v", err)
	}
	if authorization != "Bearer xoxb-test" {
		t.Fatalf("Authorization = %q, want Bearer xoxb-test", authorization)
	}
	if conversationTypes != "public_channel" {
		t.Fatalf("conversation types = %q, want least-privilege public_channel default", conversationTypes)
	}
	if len(historyChannels) != 2 || historyChannels[0] != "C123" || historyChannels[1] != "C123" {
		t.Fatalf("history channels = %v, want C123 for both pages", historyChannels)
	}
	gotReplyScopes := make(map[string]bool, len(replyScopes))
	for _, scope := range replyScopes {
		gotReplyScopes[scope] = true
	}
	if len(replyScopes) != 2 ||
		!gotReplyScopes["C123|1710000000.000001"] ||
		!gotReplyScopes["C123|1710000001.000002"] {
		t.Fatalf("reply scopes = %v, want inherited channel and message timestamps", replyScopes)
	}
	if len(sink.records) != 2 {
		t.Fatalf("records = %#v, want two thread replies only", sink.records)
	}
	for _, rec := range sink.records {
		if rec.Resource != "thread_replies" {
			t.Fatalf("resource = %q, want thread_replies (dependencies must not emit)", rec.Resource)
		}
		var data map[string]any
		if err := json.Unmarshal(rec.Data, &data); err != nil {
			t.Fatalf("decode message: %v", err)
		}
		if data["channel_id"] != "C123" {
			t.Fatalf("channel_id = %#v, want inherited C123", data["channel_id"])
		}
	}
}

func TestCredentialsFromConfigJoinsStringLists(t *testing.T) {
	got := credentialsFromConfig(filament.NewConfig(map[string]any{
		"conversation_types": []any{"public_channel", "im"},
	}), nil)
	if got["conversation_types"] != "public_channel,im" {
		t.Fatalf("conversation_types = %q, want public_channel,im", got["conversation_types"])
	}
}

func TestRequiredListConfigRejectsEmptySelection(t *testing.T) {
	src := NewManifest([]byte(`
version: 1
name: list
display_name: List
description: Test list connector.
dark_logo_url: https://cdn.example.com/list-dark.svg
light_logo_url: https://cdn.example.com/list-light.svg
config:
  choices:
    type: list
    required: true
    enum: [one, two]
connection:
  base_url: https://example.com
resources:
  - name: records
    path: /records
    records: $
    primary_key: [id]
    fields:
      id: string
`))

	if err := src.Validate(filament.NewConfig(map[string]any{"choices": []any{}})); err == nil {
		t.Fatal("empty required list validated successfully")
	}
	if err := src.Validate(filament.NewConfig(map[string]any{"choices": []any{"one"}})); err != nil {
		t.Fatalf("non-empty required list: %v", err)
	}
}

func TestEmbeddedManifestIsParsedOnceAndReused(t *testing.T) {
	src := NewManifest([]byte(`
version: 1
name: cached
display_name: Cached
description: Test cached connector.
dark_logo_url: https://cdn.example.com/cached-dark.svg
light_logo_url: https://cdn.example.com/cached-light.svg
config:
  token:
    type: secret
    required: true
connection:
  base_url: https://example.com
resources:
  - name: records
    path: /records
    records: $
    primary_key: [id]
    fields:
      id: string
`))
	if src.manifestErr != nil || src.embeddedManifest == nil {
		t.Fatalf("constructor manifest = %#v, error = %v", src.embeddedManifest, src.manifestErr)
	}

	for range 2 {
		spec := src.Spec()
		if len(spec.Config.Fields) != 1 || spec.Config.Fields[0].Name != "token" {
			t.Fatalf("config fields = %#v, want cached token field", spec.Config.Fields)
		}
	}
	cfg := filament.NewConfig(map[string]any{"token": "secret"})
	if err := src.Validate(cfg); err != nil {
		t.Fatalf("validate cached config schema: %v", err)
	}

	c, err := src.connectorForConfig(cfg)
	if err != nil {
		t.Fatalf("build connector: %v", err)
	}
	if c.manifest != src.embeddedManifest {
		t.Fatal("connector did not receive the manifest parsed by the source constructor")
	}
	if err := c.Configure(context.Background()); err != nil {
		t.Fatalf("configure with cached manifest: %v", err)
	}
	if c.manifest != src.embeddedManifest {
		t.Fatal("connector replaced the cached manifest during Configure")
	}
}

func TestEmbeddedManifestParseErrorIsCached(t *testing.T) {
	src := NewManifest([]byte("version: ["))
	if src.manifestErr == nil {
		t.Fatal("constructor accepted invalid manifest")
	}
	if err := src.Validate(filament.NewConfig(nil)); err == nil || !strings.Contains(err.Error(), "parse manifest") {
		t.Fatalf("validate error = %v, want cached manifest parse error", err)
	}
	if _, err := src.connectorForConfig(filament.NewConfig(nil)); err == nil || !strings.Contains(err.Error(), "parse manifest") {
		t.Fatalf("connector error = %v, want cached manifest parse error", err)
	}
}

func TestManifestPathIsLoadedOncePerConnector(t *testing.T) {
	path := writeTestManifest(t, "https://example.com")
	src := New()
	c, err := src.connectorForConfig(filament.NewConfig(map[string]any{"manifest_path": path}))
	if err != nil {
		t.Fatalf("build connector: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove manifest after initial load: %v", err)
	}
	if err := c.Configure(context.Background()); err != nil {
		t.Fatalf("Configure reloaded manifest instead of using parsed value: %v", err)
	}
}

func TestNewGitHubSpecAndEmbeddedManifest(t *testing.T) {
	ctx := context.Background()
	src := NewGitHub()
	spec := src.Spec()
	if spec.Name != "github" || spec.DisplayName != "GitHub" {
		t.Fatalf("spec identity = %q/%q, want github/GitHub", spec.Name, spec.DisplayName)
	}
	if len(spec.Config.Fields) != 2 {
		t.Fatalf("config fields = %#v, want token and organization", spec.Config.Fields)
	}
	fields := map[string]filament.ConfigField{}
	for _, field := range spec.Config.Fields {
		fields[field.Name] = field
	}
	if fields["token"].Type != filament.FieldSecret || !fields["token"].Required {
		t.Fatalf("token field = %#v, want required secret", fields["token"])
	}
	if fields["organization"].Type != filament.FieldString || !fields["organization"].Required {
		t.Fatalf("organization field = %#v, want required string", fields["organization"])
	}
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"token":        "github-token",
		"organization": "galaxy-io",
	})); err != nil {
		t.Fatalf("configure embedded GitHub manifest: %v", err)
	}
	defer src.Teardown(ctx)

	discovered, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	want := []string{"repositories", "issues", "pull_requests"}
	if len(discovered.Resources) != len(want) {
		t.Fatalf("resources = %#v, want %v", discovered.Resources, want)
	}
	for i := range want {
		if discovered.Resources[i].Name != want[i] {
			t.Fatalf("resource[%d] = %q, want %q", i, discovered.Resources[i].Name, want[i])
		}
	}
}

func TestStaticDiscoveryChildSelectionScansButDoesNotEmitParents(t *testing.T) {
	ctx := context.Background()
	var parentRequests, childRequests int
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/parents":
			parentRequests++
			fmt.Fprint(w, `[{"id":"parent-1"}]`)
		case "/parents/parent-1/children":
			childRequests++
			fmt.Fprint(w, `[{"id":"child-1","name":"Child"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	path := filepath.Join(t.TempDir(), "static.yaml")
	data := fmt.Sprintf(`version: 1
name: static
display_name: Static
description: Test static connector.
dark_logo_url: https://cdn.example.com/static-dark.svg
light_logo_url: https://cdn.example.com/static-light.svg
connection:
  base_url: %s
resources:
  - name: parents
    path: /parents
    primary_key: [id]
    fields:
      id: string
    capture:
      id: id
    response:
      records: $
      pagination: none
  - name: children
    path: /parents/{parent}/children
    params:
      parent: parent.id
    for_each: parents
    primary_key: [id]
    fields:
      id: string
      name: string
    response:
      records: $
      pagination: none
discovery:
  mode: static
  include: [parents, children]
`, api.URL)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"manifest_path": path})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	progress := make(chan filament.SourceProgress, 4)
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{
		Resources: []string{"children"},
		Observe:   func(p filament.SourceProgress) { progress <- p },
	}); err != nil {
		t.Fatalf("extract children: %v", err)
	}
	close(progress)
	if parentRequests != 1 || childRequests != 1 {
		t.Fatalf("requests parent=%d child=%d, want 1 each", parentRequests, childRequests)
	}
	if len(sink.records) != 1 || sink.records[0].Resource != "children" {
		t.Fatalf("emitted records = %#v, want only selected child", sink.records)
	}
	var fanOut filament.SourceProgress
	for event := range progress {
		if event.Kind == filament.SourceProgressFanOutStarted {
			fanOut = event
		}
	}
	if fanOut.Resource != "children" || fanOut.ParentsTotal != 1 {
		t.Fatalf("fan-out progress = %#v", fanOut)
	}
}

func TestSourceLinearHTTPAPIManifestExtractIssuesWithGraphQLPagination(t *testing.T) {
	ctx := context.Background()
	var sawAuth bool
	var afterValues []any
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "linear-token" {
			sawAuth = true
		}
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode graphql body: %v", err)
		}
		if !strings.Contains(body.Query, "query Issues") {
			t.Fatalf("unexpected query: %s", body.Query)
		}
		afterValues = append(afterValues, body.Variables["after"])
		w.Header().Set("Content-Type", "application/json")
		if body.Variables["after"] == nil {
			fmt.Fprint(w, `{
				"data":{"issues":{
					"nodes":[{
						"id":"issue1","identifier":"ENG-1","number":1,"title":"Bridge Linear","description":"sync it","priority":2,
						"estimate":3,"url":"https://linear.app/acme/issue/ENG-1","branchName":"eng-1",
						"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z",
						"completedAt":null,"archivedAt":null,"canceledAt":null,
						"team":{"id":"team1","key":"ENG","name":"Engineering"},
						"state":{"id":"state1","name":"Todo","type":"unstarted"},
						"assignee":{"id":"user1","name":"Ada","displayName":"Ada","email":"ada@example.com"},
						"creator":{"id":"user2","name":"Grace","displayName":"Grace","email":"grace@example.com"},
						"project":{"id":"project1","name":"Ingestion"},
						"labels":{"nodes":[{"id":"label1","name":"api","color":"#111111"}]}
					}],
					"pageInfo":{"hasNextPage":true,"endCursor":"cursor-1"}
				}}
			}`)
			return
		}
		if body.Variables["after"] != "cursor-1" {
			t.Fatalf("after = %v, want cursor-1", body.Variables["after"])
		}
		fmt.Fprint(w, `{"data":{"issues":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":null}}}}`)
	}))
	defer api.Close()

	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"manifest_path": writeLinearTestManifest(t, api.URL),
		"api_key":       "linear-token",
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	discovered, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(discovered.Resources) != 4 {
		t.Fatalf("resources = %#v, want manifest resources", discovered.Resources)
	}
	schema, err := src.Schema(ctx, "issues")
	if err != nil {
		t.Fatalf("schema issues: %v", err)
	}
	fields := map[string]filament.LogicalType{}
	for _, field := range schema.Fields {
		fields[field.Name] = field.Logical
	}
	if fields["created_at"] != filament.LogicalTimestampTZ || fields["priority"] != filament.LogicalInt64 ||
		fields["labels"] != filament.LogicalJSON || fields["assignee"] != filament.LogicalJSON || fields["creator"] != filament.LogicalJSON {
		t.Fatalf("issue schema fields = %#v", fields)
	}

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"issues"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !sawAuth {
		t.Fatal("Linear API key Authorization header was not applied")
	}
	if len(afterValues) != 2 || afterValues[0] != nil || afterValues[1] != "cursor-1" {
		t.Fatalf("after values = %#v, want nil then cursor-1", afterValues)
	}
	if len(sink.records) != 1 {
		t.Fatalf("records = %d, want 1", len(sink.records))
	}
	rec := sink.records[0]
	if rec.Resource != "issues" || rec.ID != "issue1" {
		t.Fatalf("record = %s/%s, want issues/issue1", rec.Resource, rec.ID)
	}
	var data map[string]any
	if err := json.Unmarshal(rec.Data, &data); err != nil {
		t.Fatalf("record data json: %v", err)
	}
	if data["identifier"] != "ENG-1" || data["team_id"] != "team1" || data["created_at"] != "2026-01-01T00:00:00Z" {
		t.Fatalf("projected data = %#v", data)
	}
	assignee, ok := data["assignee"].(map[string]any)
	if !ok {
		t.Fatalf("assignee = %#v, want shaped object", data["assignee"])
	}
	if assignee["id"] != "user1" || assignee["display_name"] != "Ada" || assignee["email"] != "ada@example.com" {
		t.Fatalf("assignee shape = %#v", assignee)
	}
	creator, ok := data["creator"].(map[string]any)
	if !ok || creator["id"] != "user2" || creator["display_name"] != "Grace" {
		t.Fatalf("creator shape = %#v", data["creator"])
	}
	raw, ok := data["raw"].(map[string]any)
	if !ok {
		t.Fatalf("raw = %#v, want object", data["raw"])
	}
	if _, ok := raw["identifier"]; ok {
		t.Fatalf("raw duplicated projected identifier: %#v", raw)
	}
	for _, key := range []string{"team", "state", "assignee", "creator", "project", "labels"} {
		if _, ok := raw[key]; ok {
			t.Fatalf("raw duplicated projected %s: %#v", key, raw)
		}
	}
	if _, ok := data["data"]; ok {
		t.Fatalf("record still has generic data envelope: %#v", data)
	}
}

// testRecord is one row as the tests inspect it: the resource, the primary key
// value (a single key column's text), the row rendered as JSON, and its cursor.
type testRecord struct {
	Resource string
	ID       string
	Data     []byte
	Key      []string
}

// collectSink is an Arrow inlet that renders every flushed row back to
// a testRecord.
type collectSink struct {
	records []testRecord
}

func (s *collectSink) Builder(resource string, _ int, schema rowmodel.Schema) (arrowbatch.RowWriter, error) {
	as := arrowbatch.Schema(schema)
	return arrowbatch.NewBuilder(as, nil, arrowbatch.Options{MaxRows: 1}, &collectChunks{sink: s, resource: resource, pk: schema.PrimaryKey, enc: ndjson.NewEncoder(as)}), nil
}

type collectChunks struct {
	sink     *collectSink
	resource string
	pk       []string
	enc      *ndjson.Encoder
}

func (c *collectChunks) Chunk(ch *arrowbatch.Batch) error {
	defer ch.Release()
	rows := ch.Rows()
	for i := range int(rows.NumRows()) {
		rec := testRecord{Resource: c.resource, Data: c.enc.AppendRow(nil, rows, i), Key: ch.Last.Key}
		if len(c.pk) == 1 {
			if idx := rows.Schema().FieldIndices(c.pk[0]); len(idx) == 1 {
				rec.ID = rows.Column(idx[0]).ValueStr(i)
			}
		}
		c.sink.records = append(c.sink.records, rec)
	}
	return nil
}

func (c *collectChunks) Drained(rowmodel.Meta, int) error { return nil }

func writeTestManifest(t *testing.T, baseURL string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	data := fmt.Sprintf(`version: 1
name: test_http
display_name: Test HTTP
description: Test HTTP connector.
dark_logo_url: https://cdn.example.com/http-dark.svg
light_logo_url: https://cdn.example.com/http-light.svg
connection:
  base_url: %s
resources:
  - name: items
    path: /items
    method: GET
    primary_key: [id]
    response:
      records: $.items
      pagination:
        cursor:
          response: next_cursor
          request: query.start_cursor
          more: has_more
`, baseURL)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func writeIncrementalTestManifest(t *testing.T, baseURL string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	data := fmt.Sprintf(`version: 1
name: test_http
display_name: Test HTTP
description: Test incremental HTTP connector.
dark_logo_url: https://cdn.example.com/http-dark.svg
light_logo_url: https://cdn.example.com/http-light.svg
connection:
  base_url: %s
resources:
  - name: items
    path: /items
    method: GET
    primary_key: [id]
    fields:
      id: string
      updated_at: timestamptz
    response:
      records: $.items
      pagination:
        cursor:
          response: next_cursor
          request: query.start_cursor
          more: has_more
    incremental:
      cursor_field: updated_at
      start_param: since
      inject_into: query
      checkpoint_key: items_since
      comparator: time
`, baseURL)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func writeLinearTestManifest(t *testing.T, baseURL string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "linear.yaml")
	data := strings.Replace(string(linearManifest), "https://api.linear.app", baseURL, 1)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write linear manifest: %v", err)
	}
	return path
}

func writeNotionTestManifest(t *testing.T, baseURL string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notion.yaml")
	data := fmt.Sprintf(`version: 1
name: notion
display_name: Notion
description: Test Notion connector.
dark_logo_url: https://cdn.example.com/notion-dark.svg
light_logo_url: https://cdn.example.com/notion-light.svg
connection:
  base_url: %s
  auth:
    bearer: config.api_key
  headers:
    Notion-Version: "2022-06-28"
resources:
  - name: databases
    path: /v1/search
    method: POST
    primary_key: [id]
    fields:
      id: string
      object: string
      created_time: timestamptz?
      last_edited_time: timestamptz?
      title: { path: title.0.plain_text, type: string, nullable: true }
      raw: { path: $, type: json }
    capture:
      database_id: id
    body:
      encoding: json
      template:
        filter:
          property: object
          value: database
    response:
      records: $.results
      pagination:
        cursor:
          response: next_cursor
          request: body.start_cursor
          more: has_more
  - name: pages
    emit_as: "pages_{{ parent.database_id }}"
    path: /v1/databases/{{ parent.database_id }}/query
    method: POST
    primary_key: [id]
    fields:
      id: string
      database_id: { path: parent.database_id, type: string }
      object: string
      created_time: timestamptz?
      last_edited_time: timestamptz?
      archived: bool?
      url: string?
      properties: json?
      raw: { path: $, type: json }
    parent:
      resource: databases
    response:
      records: $.results
      pagination:
        cursor:
          response: next_cursor
          request: body.start_cursor
          more: has_more
discovery:
  mode: dynamic
  resources:
    - from: databases
      map:
        kind: database
        id: id
        name: title.0.plain_text
        metadata:
          object: object
`, baseURL)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write notion manifest: %v", err)
	}
	return path
}

func TestNewResendSpecAndEmbeddedManifest(t *testing.T) {
	ctx := context.Background()
	src := NewResend()
	spec := src.Spec()
	if spec.Name != "resend" || spec.DisplayName != "Resend" {
		t.Fatalf("spec identity = %q/%q, want resend/Resend", spec.Name, spec.DisplayName)
	}
	fields := map[string]filament.ConfigField{}
	for _, field := range spec.Config.Fields {
		fields[field.Name] = field
	}
	if len(fields) != 1 {
		t.Fatalf("config fields = %#v, want the API key alone", spec.Config.Fields)
	}
	if fields["api_key"].Type != filament.FieldSecret || !fields["api_key"].Required {
		t.Fatalf("api_key field = %#v, want required secret", fields["api_key"])
	}
	if err := src.Validate(filament.NewConfig(map[string]any{})); err == nil {
		t.Fatal("validate without API key succeeded")
	}
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "re_test_123"})); err != nil {
		t.Fatalf("configure embedded Resend manifest: %v", err)
	}
	defer src.Teardown(ctx)

	discovered, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	// No metrics or broadcast-recipient resources: those endpoints are private
	// beta and 404 for accounts outside it, which fails the whole run.
	want := []string{
		"emails", "email_details", "email_attachments", "received_emails",
		"received_email_attachments", "domains", "domain_settings", "domain_records",
		"api_keys", "broadcasts", "contacts", "contact_topics", "contact_properties",
		"segments", "segment_contacts", "topics", "suppressions", "webhooks",
		"webhook_events", "webhook_event_attempts", "templates", "template_details",
		"logs",
	}
	if len(discovered.Resources) != len(want) {
		t.Fatalf("resources = %d, want %d", len(discovered.Resources), len(want))
	}
	for i, name := range want {
		if discovered.Resources[i].Name != name {
			t.Fatalf("resource[%d] = %q, want %q", i, discovered.Resources[i].Name, name)
		}
	}
}

// Resend returns no next-page token: the cursor is the id of the last record
// on the page, echoed back as `after`, with has_more as the terminator.
func TestResendPaginatesOnLastRecordIDAndSendsBearerToken(t *testing.T) {
	ctx := context.Background()
	var auth string
	var afters, limits []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/emails" {
			http.NotFound(w, r)
			return
		}
		auth = r.Header.Get("Authorization")
		afters = append(afters, r.URL.Query().Get("after"))
		limits = append(limits, r.URL.Query().Get("limit"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("after") == "" {
			fmt.Fprint(w, `{"object":"list","has_more":true,"data":[
				{"id":"em_1","message_id":"<1@example.com>","to":["a@example.com"],"from":"Acme <s@example.com>",
				 "subject":"One","last_event":"delivered","bcc":null,"cc":null,"reply_to":null,
				 "created_at":"2026-04-03 22:13:42.674981+00","scheduled_at":null},
				{"id":"em_2","message_id":"<2@example.com>","to":["b@example.com"],"from":"Acme <s@example.com>",
				 "subject":"Two","last_event":"opened","bcc":null,"cc":null,"reply_to":null,
				 "created_at":"2026-04-03 22:14:42.674981+00","scheduled_at":null}]}`)
			return
		}
		fmt.Fprint(w, `{"object":"list","has_more":false,"data":[
			{"id":"em_3","message_id":"<3@example.com>","to":["c@example.com"],"from":"Acme <s@example.com>",
			 "subject":"Three","last_event":"bounced","bcc":null,"cc":null,"reply_to":null,
			 "created_at":"2026-04-03 22:15:42.674981+00","scheduled_at":null}]}`)
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(resendManifest), "https://api.resend.com", api.URL, 1))
	src := NewManifest(manifestData)
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "re_test_123"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"emails"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract emails: %v", err)
	}

	if auth != "Bearer re_test_123" {
		t.Fatalf("authorization = %q, want the API key as a bearer token", auth)
	}
	if len(afters) != 2 || afters[0] != "" || afters[1] != "em_2" {
		t.Fatalf("after = %v, want the last id of page one on the second request", afters)
	}
	for i, limit := range limits {
		if limit != "100" {
			t.Fatalf("limit[%d] = %q, want Resend's 100 maximum", i, limit)
		}
	}
	if len(sink.records) != 3 {
		t.Fatalf("records = %d, want all three emails across both pages", len(sink.records))
	}
	var first map[string]any
	if err := json.Unmarshal(sink.records[0].Data, &first); err != nil {
		t.Fatalf("decode email: %v", err)
	}
	if first["id"] != "em_1" || first["subject"] != "One" || first["last_event"] != "delivered" {
		t.Fatalf("email projection = %#v", first)
	}
	// Postgres-rendered timestamps ("+00", space separator) are not RFC 3339,
	// so created_at stays a string rather than a timestamptz the sinks would
	// fail to parse.
	if first["created_at"] != "2026-04-03 22:13:42.674981+00" {
		t.Fatalf("created_at = %#v, want the raw Postgres-rendered value preserved", first["created_at"])
	}
	if _, ok := first["raw"].(map[string]any); !ok {
		t.Fatalf("raw remainder = %#v, want the unmapped payload", first["raw"])
	}
}

// TestConnection probes the first top-level, non-streaming resource with only
// the config scope bound, so `emails` has to be declared first and has to need
// no parent capture.
func TestResendTestConnectionProbesEmails(t *testing.T) {
	var paths []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"list","has_more":true,"data":[{"id":"em_1"}]}`)
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(resendManifest), "https://api.resend.com", api.URL, 1))
	src := NewManifest(manifestData)
	if err := src.TestConnection(context.Background(), filament.NewConfig(map[string]any{
		"api_key": "re_test_123",
	})); err != nil {
		t.Fatalf("test connection: %v", err)
	}
	// has_more is true, but validation must not walk pagination.
	if len(paths) != 1 || paths[0] != "/emails" {
		t.Fatalf("probe requests = %v, want exactly one GET /emails", paths)
	}
}

// The detail endpoints return a bare object rather than a list envelope, and
// take no list parameters — `records: $` plus `cardinality: one` has to yield
// exactly one row from one unpaginated request.
func TestResendEmailDetailsSingletonFanOutSendsNoListParams(t *testing.T) {
	ctx := context.Background()
	var detailQueries []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/emails":
			fmt.Fprint(w, `{"object":"list","has_more":false,"data":[
				{"id":"em_1","from":"s@example.com","subject":"One","created_at":"2026-04-03 22:13:42.674981+00"}]}`)
		case "/emails/em_1":
			detailQueries = append(detailQueries, r.URL.RawQuery)
			fmt.Fprint(w, `{"object":"email","id":"em_1","message_id":"<1@example.com>","to":["a@example.com"],
				"from":"Acme <s@example.com>","subject":"One","html":"<p>Hi</p>","text":null,"bcc":[],"cc":[],
				"reply_to":[],"last_event":"delivered","scheduled_at":null,
				"created_at":"2026-04-03 22:13:42.674981+00","tags":[{"name":"category","value":"welcome"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(resendManifest), "https://api.resend.com", api.URL, 1))
	src := NewManifest(manifestData)
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "re_test_123"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"email_details"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract email details: %v", err)
	}
	if len(detailQueries) != 1 {
		t.Fatalf("detail requests = %v, want a single unpaginated fetch per email", detailQueries)
	}
	if detailQueries[0] != "" {
		t.Fatalf("detail query = %q, want no list parameters on /emails/{id}", detailQueries[0])
	}
	if len(sink.records) != 1 {
		t.Fatalf("records = %d, want the email object as one row and no parent rows", len(sink.records))
	}
	var data map[string]any
	if err := json.Unmarshal(sink.records[0].Data, &data); err != nil {
		t.Fatalf("decode email detail: %v", err)
	}
	if data["id"] != "em_1" || data["html"] != "<p>Hi</p>" || data["text"] != nil {
		t.Fatalf("email detail projection = %#v", data)
	}
	if tags, ok := data["tags"].([]any); !ok || len(tags) != 1 {
		t.Fatalf("tags = %#v, want the send-time tags the list endpoint omits", data["tags"])
	}
}

// domain_records projects the nested DNS array out of the same detail payload
// domain_settings reads as a single object.
func TestResendDomainRecordsProjectNestedArrayFromDomainDetail(t *testing.T) {
	ctx := context.Background()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/domains":
			fmt.Fprint(w, `{"object":"list","has_more":false,"data":[
				{"id":"dom_1","name":"example.com","status":"verified","region":"us-east-1",
				 "created_at":"2026-04-26 20:21:26.347412+00","capabilities":{"sending":"enabled","receiving":"disabled"}}]}`)
		case "/domains/dom_1":
			fmt.Fprint(w, `{"object":"domain","id":"dom_1","name":"example.com","status":"verified",
				"region":"us-east-1","open_tracking":true,"click_tracking":false,"tracking_subdomain":"links",
				"created_at":"2026-04-26 20:21:26.347412+00",
				"capabilities":{"sending":"enabled","receiving":"disabled"},
				"records":[
					{"record":"SPF","name":"send","type":"MX","ttl":"Auto","status":"verified",
					 "value":"feedback-smtp.us-east-1.amazonses.com","priority":10},
					{"record":"DKIM","name":"resend._domainkey","type":"TXT","ttl":"Auto",
					 "status":"verified","value":"p=MIGf"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(resendManifest), "https://api.resend.com", api.URL, 1))
	src := NewManifest(manifestData)
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "re_test_123"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var records collectSink
	if err := src.Extract(ctx, &records, filament.ExtractOpts{Resources: []string{"domain_records"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract domain records: %v", err)
	}
	if len(records.records) != 2 {
		t.Fatalf("records = %d, want one row per DNS record", len(records.records))
	}
	var spf map[string]any
	if err := json.Unmarshal(records.records[0].Data, &spf); err != nil {
		t.Fatalf("decode dns record: %v", err)
	}
	if spf["domain_id"] != "dom_1" || spf["record"] != "SPF" || spf["type"] != "MX" {
		t.Fatalf("dns record projection = %#v", spf)
	}
	if spf["priority"] != float64(10) {
		t.Fatalf("priority = %#v, want the MX priority as an integer", spf["priority"])
	}
	var dkim map[string]any
	if err := json.Unmarshal(records.records[1].Data, &dkim); err != nil {
		t.Fatalf("decode dns record: %v", err)
	}
	if dkim["priority"] != nil {
		t.Fatalf("priority = %#v, want null on a record type that carries none", dkim["priority"])
	}

	// A fresh Source: parent captures accumulate for the lifetime of a
	// configured connector, so reusing the one above would fan the detail
	// request out once per prior run.
	settingsSrc := NewManifest(manifestData)
	if err := settingsSrc.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "re_test_123"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer settingsSrc.Teardown(ctx)

	var settings collectSink
	if err := settingsSrc.Extract(ctx, &settings, filament.ExtractOpts{Resources: []string{"domain_settings"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract domain settings: %v", err)
	}
	if len(settings.records) != 1 {
		t.Fatalf("settings records = %d, want one row per domain", len(settings.records))
	}
	var detail map[string]any
	if err := json.Unmarshal(settings.records[0].Data, &detail); err != nil {
		t.Fatalf("decode domain settings: %v", err)
	}
	if detail["open_tracking"] != true || detail["tracking_subdomain"] != "links" {
		t.Fatalf("domain settings projection = %#v, want the tracking fields the list endpoint omits", detail)
	}
}

// webhook_event_attempts is the only three-level fan-out in the manifest: the
// webhook id has to survive from the grandparent through the event capture.
func TestResendWebhookAttemptsInheritWebhookIDThroughEventCapture(t *testing.T) {
	ctx := context.Background()
	var attemptPaths []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/webhooks":
			fmt.Fprint(w, `{"object":"list","has_more":false,"data":[
				{"id":"wh_1","endpoint":"https://example.com/hook","status":"enabled",
				 "events":["email.sent"],"created_at":"2026-09-10 10:15:30.000+00"}]}`)
		case "/webhooks/wh_1/events":
			fmt.Fprint(w, `{"object":"list","has_more":false,"data":[
				{"id":"msg_1","type":"email.sent","created_at":"2026-08-22T15:28:00.000Z","status":"success"},
				{"id":"msg_2","type":"email.delivered","created_at":"2026-08-22T15:27:42.000Z","status":"failed"}]}`)
		case "/webhooks/wh_1/events/msg_1/attempts", "/webhooks/wh_1/events/msg_2/attempts":
			attemptPaths = append(attemptPaths, r.URL.Path)
			fmt.Fprint(w, `{"object":"list","has_more":false,"data":[
				{"id":"atmpt_1","http_status_code":200,"response":"{\"ok\":true}","sent_at":"2026-08-22T15:33:12.000Z"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(resendManifest), "https://api.resend.com", api.URL, 1))
	src := NewManifest(manifestData)
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "re_test_123"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"webhook_event_attempts"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract webhook attempts: %v", err)
	}
	if len(attemptPaths) != 2 {
		t.Fatalf("attempt requests = %v, want one per event", attemptPaths)
	}
	if len(sink.records) != 2 {
		t.Fatalf("records = %d, want one attempt per event and no ancestor rows", len(sink.records))
	}
	seen := map[string]bool{}
	for _, rec := range sink.records {
		var data map[string]any
		if err := json.Unmarshal(rec.Data, &data); err != nil {
			t.Fatalf("decode attempt: %v", err)
		}
		if data["webhook_id"] != "wh_1" {
			t.Fatalf("webhook_id = %#v, want the grandparent id carried through the event capture", data["webhook_id"])
		}
		if data["http_status_code"] != float64(200) {
			t.Fatalf("http_status_code = %#v, want the delivery status as an integer", data["http_status_code"])
		}
		if data["sent_at"] != "2026-08-22T15:33:12Z" { // a typed timestamp, rendered without the source's zero millis
			t.Fatalf("sent_at = %#v, want the ISO-8601 attempt timestamp", data["sent_at"])
		}
		seen[data["event_id"].(string)] = true
	}
	if !seen["msg_1"] || !seen["msg_2"] {
		t.Fatalf("event_id values = %v, want both events denormalized onto their attempts", seen)
	}
}

// reply_to is documented "string | string[]" and shows as null in the docs
// example, but live templates return an array. Scalar-typed fields abort the
// whole child extraction on a type mismatch — nullable only covers missing and
// null — so the address fields have to be json.
func TestResendTemplateDetailsAcceptArrayReplyTo(t *testing.T) {
	ctx := context.Background()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/templates":
			fmt.Fprint(w, `{"object":"list","has_more":false,"data":[
				{"id":"tpl_1","name":"reset-password","alias":"reset-password","status":"published",
				 "published_at":"2026-10-06 23:47:56.678+00","created_at":"2026-10-06 23:47:56.678+00",
				 "updated_at":"2026-10-06 23:47:56.678+00"}]}`)
		case "/templates/tpl_1":
			fmt.Fprint(w, `{"object":"template","id":"tpl_1","current_version_id":"ver_1",
				"alias":"reset-password","name":"reset-password","status":"published",
				"published_at":"2026-10-06 23:47:56.678+00","created_at":"2026-10-06 23:47:56.678+00",
				"updated_at":"2026-10-06 23:47:56.678+00","from":"John Doe <john@example.com>",
				"subject":"Hello","reply_to":["support@example.com","ops@example.com"],
				"html":"<h1>Hello</h1>","text":"Hello","has_unpublished_versions":true,
				"variables":[{"id":"var_1","key":"user_name","type":"string"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(resendManifest), "https://api.resend.com", api.URL, 1))
	src := NewManifest(manifestData)
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "re_test_123"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"template_details"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract template details: %v", err)
	}
	if len(sink.records) != 1 {
		t.Fatalf("records = %d, want one row per template", len(sink.records))
	}
	var data map[string]any
	if err := json.Unmarshal(sink.records[0].Data, &data); err != nil {
		t.Fatalf("decode template detail: %v", err)
	}
	replyTo, ok := data["reply_to"].([]any)
	if !ok || len(replyTo) != 2 || replyTo[0] != "support@example.com" {
		t.Fatalf("reply_to = %#v, want the multi-address array preserved", data["reply_to"])
	}
	if data["from"] != "John Doe <john@example.com>" {
		t.Fatalf("from = %#v, want the single sender as a scalar", data["from"])
	}
	if data["html"] != "<h1>Hello</h1>" {
		t.Fatalf("html = %#v, want the rendered body the list endpoint omits", data["html"])
	}
}

func TestNewStripeSpecAndEmbeddedManifest(t *testing.T) {
	ctx := context.Background()
	src := NewStripe()
	spec := src.Spec()
	if spec.Name != "stripe" || spec.DisplayName != "Stripe" {
		t.Fatalf("spec identity = %q/%q, want stripe/Stripe", spec.Name, spec.DisplayName)
	}
	fields := map[string]filament.ConfigField{}
	for _, field := range spec.Config.Fields {
		fields[field.Name] = field
	}
	if len(fields) != 1 {
		t.Fatalf("config fields = %#v, want the API key alone", spec.Config.Fields)
	}
	if fields["api_key"].Type != filament.FieldSecret || !fields["api_key"].Required {
		t.Fatalf("api_key field = %#v, want required secret", fields["api_key"])
	}
	if err := src.Validate(filament.NewConfig(map[string]any{})); err == nil {
		t.Fatal("validate without API key succeeded")
	}
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "rk_test_123"})); err != nil {
		t.Fatalf("configure embedded Stripe manifest: %v", err)
	}
	defer src.Teardown(ctx)

	discovered, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	// No Connect or platform resources: transfers, application fees, top-ups,
	// quotes, payment links and Radar warnings are gated on products the
	// account may not have enabled.
	want := []string{
		"customers", "charges", "payment_intents", "refunds", "disputes",
		"balance_transactions", "payouts", "invoices", "invoice_line_items",
		"credit_notes", "subscriptions", "subscription_items", "products",
		"prices", "coupons", "promotion_codes", "checkout_sessions",
		"setup_intents", "payment_methods", "tax_rates", "events",
	}
	if len(discovered.Resources) != len(want) {
		t.Fatalf("resources = %d, want %d", len(discovered.Resources), len(want))
	}
	for i, name := range want {
		if discovered.Resources[i].Name != name {
			t.Fatalf("resource[%d] = %q, want %q", i, discovered.Resources[i].Name, name)
		}
	}
}

// Stripe returns no next-page token: the cursor is the id of the last record on
// the page, echoed back as `starting_after`, with has_more as the terminator.
// Auth is the API key as the HTTP basic username with an empty password.
func TestStripePaginatesOnLastRecordIDAndSendsBasicAuth(t *testing.T) {
	ctx := context.Background()
	var user, pass string
	var okBasic bool
	var version string
	var startingAfters, limits []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/charges" {
			http.NotFound(w, r)
			return
		}
		user, pass, okBasic = r.BasicAuth()
		version = r.Header.Get("Stripe-Version")
		startingAfters = append(startingAfters, r.URL.Query().Get("starting_after"))
		limits = append(limits, r.URL.Query().Get("limit"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("starting_after") == "" {
			fmt.Fprint(w, `{"object":"list","url":"/v1/charges","has_more":true,"data":[
				{"id":"ch_1","object":"charge","amount":1099,"amount_refunded":0,"currency":"usd",
				 "created":1679090539,"customer":"cus_1","captured":true,"paid":true,"refunded":false,
				 "status":"succeeded","livemode":false,"metadata":{},"failure_code":null,
				 "receipt_url":"https://pay.stripe.com/receipts/1"},
				{"id":"ch_2","object":"charge","amount":2200,"amount_refunded":2200,"currency":"usd",
				 "created":1679090600,"customer":null,"captured":true,"paid":true,"refunded":true,
				 "status":"succeeded","livemode":false,"metadata":{},"failure_code":null,
				 "receipt_url":null}]}`)
			return
		}
		fmt.Fprint(w, `{"object":"list","url":"/v1/charges","has_more":false,"data":[
			{"id":"ch_3","object":"charge","amount":500,"amount_refunded":0,"currency":"eur",
			 "created":1679090700,"customer":"cus_2","captured":false,"paid":false,"refunded":false,
			 "status":"failed","livemode":false,"metadata":{},"failure_code":"card_declined",
			 "receipt_url":null}]}`)
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(stripeManifest), "https://api.stripe.com", api.URL, 1))
	src := NewManifest(manifestData)
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "rk_test_123"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"charges"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract charges: %v", err)
	}

	if !okBasic || user != "rk_test_123" || pass != "" {
		t.Fatalf("basic auth = %q/%q (ok=%v), want the API key as username with no password", user, pass, okBasic)
	}
	if version != "2026-07-29.dahlia" {
		t.Fatalf("Stripe-Version = %q, want the pinned API version", version)
	}
	if len(startingAfters) != 2 || startingAfters[0] != "" || startingAfters[1] != "ch_2" {
		t.Fatalf("starting_after = %v, want the last id of page one on the second request", startingAfters)
	}
	for i, limit := range limits {
		if limit != "100" {
			t.Fatalf("limit[%d] = %q, want Stripe's 100 maximum rather than the default 10", i, limit)
		}
	}
	if len(sink.records) != 3 {
		t.Fatalf("records = %d, want all three charges across both pages", len(sink.records))
	}
	var first map[string]any
	if err := json.Unmarshal(sink.records[0].Data, &first); err != nil {
		t.Fatalf("decode charge: %v", err)
	}
	if first["id"] != "ch_1" || first["status"] != "succeeded" || first["currency"] != "usd" {
		t.Fatalf("charge projection = %#v", first)
	}
	// Stripe timestamps are Unix epoch integers, not RFC 3339, so created is an
	// int64 column rather than a timestamptz the sinks would fail to coerce.
	if created, ok := first["created"].(float64); !ok || int64(created) != 1679090539 {
		t.Fatalf("created = %#v, want the epoch integer preserved", first["created"])
	}
	if _, ok := first["raw"].(map[string]any); !ok {
		t.Fatalf("raw remainder = %#v, want the unmapped payload", first["raw"])
	}
}

// /v1/subscription_items requires a subscription, so it fans out from
// subscriptions with the parent id templated into the query. The parent must
// send status=all or canceled subscriptions never appear.
func TestStripeSubscriptionItemFanOut(t *testing.T) {
	ctx := context.Background()
	var subscriptionStatus string
	var itemScopes []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/subscriptions":
			subscriptionStatus = r.URL.Query().Get("status")
			fmt.Fprint(w, `{"object":"list","has_more":false,"data":[
				{"id":"sub_1","object":"subscription","created":1679609767,"customer":"cus_1",
				 "status":"active","currency":"usd","livemode":false,"metadata":{},
				 "cancel_at_period_end":false,"start_date":1679609767},
				{"id":"sub_2","object":"subscription","created":1679609800,"customer":"cus_2",
				 "status":"canceled","currency":"usd","livemode":false,"metadata":{},
				 "cancel_at_period_end":false,"canceled_at":1679700000,"start_date":1679609800}]}`)
		case "/v1/subscription_items":
			scope := r.URL.Query().Get("subscription")
			itemScopes = append(itemScopes, scope)
			fmt.Fprintf(w, `{"object":"list","has_more":false,"data":[
				{"id":"si_%s","object":"subscription_item","created":1679609768,"quantity":1,
				 "subscription":"%s","current_period_start":1679609767,"current_period_end":1682288167,
				 "metadata":{},"tax_rates":[]}]}`, scope, scope)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(stripeManifest), "https://api.stripe.com", api.URL, 1))
	src := NewManifest(manifestData)
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "rk_test_123"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"subscription_items"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract subscription items: %v", err)
	}

	if subscriptionStatus != "all" {
		t.Fatalf("subscription status filter = %q, want all so canceled subscriptions are not dropped", subscriptionStatus)
	}
	gotScopes := map[string]bool{}
	for _, scope := range itemScopes {
		gotScopes[scope] = true
	}
	if len(itemScopes) != 2 || !gotScopes["sub_1"] || !gotScopes["sub_2"] {
		t.Fatalf("item scopes = %v, want one request per parent subscription", itemScopes)
	}
	if len(sink.records) != 2 {
		t.Fatalf("records = %d, want one item per subscription", len(sink.records))
	}
	for _, rec := range sink.records {
		if rec.Resource != "subscription_items" {
			t.Fatalf("resource = %q, want subscription_items (dependencies must not emit)", rec.Resource)
		}
		var data map[string]any
		if err := json.Unmarshal(rec.Data, &data); err != nil {
			t.Fatalf("decode subscription item: %v", err)
		}
		// Recent API versions carry the billing period on the item, not the
		// subscription, so this is the only place it surfaces.
		if _, ok := data["current_period_end"].(float64); !ok {
			t.Fatalf("current_period_end = %#v, want the item-level billing period", data["current_period_end"])
		}
	}
}

// The invoice's embedded `lines` truncates at 10 with its own has_more, so line
// detail comes from a per-invoice fan-out that denormalizes the parent id.
func TestStripeInvoiceLineItemFanOut(t *testing.T) {
	ctx := context.Background()
	var linePaths []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/invoices":
			fmt.Fprint(w, `{"object":"list","has_more":false,"data":[
				{"id":"in_1","object":"invoice","created":1680644467,"customer":"cus_1",
				 "currency":"usd","status":"paid","total":1099,"amount_due":1099,"amount_paid":1099,
				 "livemode":false,"metadata":{},"lines":{"object":"list","has_more":true,"data":[]}},
				{"id":"in_2","object":"invoice","created":1680644500,"customer":"cus_2",
				 "currency":"usd","status":"draft","total":0,"amount_due":0,"amount_paid":0,
				 "livemode":false,"metadata":{},"lines":{"object":"list","has_more":false,"data":[]}}]}`)
		case strings.HasSuffix(r.URL.Path, "/lines"):
			linePaths = append(linePaths, r.URL.Path)
			fmt.Fprint(w, `{"object":"list","has_more":false,"data":[
				{"id":"il_1","object":"line_item","amount":1099,"currency":"usd",
				 "description":"T-shirt","quantity":1,"discountable":true,"livemode":false,
				 "metadata":{},"period":{"start":1680644467,"end":1680644467}}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(stripeManifest), "https://api.stripe.com", api.URL, 1))
	src := NewManifest(manifestData)
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "rk_test_123"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"invoice_line_items"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract invoice line items: %v", err)
	}

	gotPaths := map[string]bool{}
	for _, path := range linePaths {
		gotPaths[path] = true
	}
	if len(linePaths) != 2 || !gotPaths["/v1/invoices/in_1/lines"] || !gotPaths["/v1/invoices/in_2/lines"] {
		t.Fatalf("line paths = %v, want one request per parent invoice", linePaths)
	}
	if len(sink.records) != 2 {
		t.Fatalf("records = %d, want one line per invoice", len(sink.records))
	}
	invoiceIDs := map[string]bool{}
	for _, rec := range sink.records {
		var data map[string]any
		if err := json.Unmarshal(rec.Data, &data); err != nil {
			t.Fatalf("decode invoice line: %v", err)
		}
		id, _ := data["invoice_id"].(string)
		invoiceIDs[id] = true
		// Line items carry no created timestamp, so the shared field set
		// excludes it rather than aborting the run on a missing path.
		if _, ok := data["created"]; ok {
			t.Fatalf("created = %#v, want the column excluded for line items", data["created"])
		}
	}
	if !invoiceIDs["in_1"] || !invoiceIDs["in_2"] {
		t.Fatalf("invoice_id values = %v, want the parent id denormalized onto each line", invoiceIDs)
	}
}

// Stripe's only incremental filter is the bracketed `created[gte]`, which Go
// percent-encodes to created%5Bgte%5D on the wire. This pins that round trip
// along with the numeric comparator: created is an epoch integer, and a naive
// float stringification would send 1.679090539e+09 and match nothing.
func TestStripeIncrementalInjectsBracketedCreatedFilter(t *testing.T) {
	ctx := context.Background()
	var gotCreatedGte string
	var rawQuery string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/charges" {
			http.NotFound(w, r)
			return
		}
		gotCreatedGte = r.URL.Query().Get("created[gte]")
		rawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"list","url":"/v1/charges","has_more":false,"data":[
			{"id":"ch_9","object":"charge","amount":1000,"currency":"usd","created":1679090700,
			 "status":"succeeded","livemode":false,"metadata":{}}]}`)
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(stripeManifest), "https://api.stripe.com", api.URL, 1))
	src := NewManifest(manifestData)
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "rk_test_123"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	// Each ledger resource needs its own checkpoint key; the cursor_field
	// fallback would collide across all ten and fail manifest validation.
	prev := map[string]filament.Checkpoint{
		"charges": checkpoint.KeysetCheckpoint{
			Mode:   checkpoint.ModeIncremental,
			Cols:   []string{"charges_created"},
			Types:  []string{"int64"},
			Shards: []checkpoint.KeysetShard{{Key: []string{"1679090539"}}},
		}.ToCheckpoint("charges"),
	}
	plan, err := src.PlanIncremental(ctx, []string{"charges"}, prev, map[string]filament.ResourceCursorConfig{
		"charges": {Field: "created"},
	})
	if err != nil {
		t.Fatalf("plan incremental: %v", err)
	}
	ks, ok := checkpoint.ParseKeyset(plan["charges"])
	if !ok {
		t.Fatal("plan did not parse as keyset")
	}
	if got, want := ks.Cols, []string{"charges_created"}; !slices.Equal(got, want) {
		t.Fatalf("checkpoint cols = %v, want durable watermark only %v", got, want)
	}
	var sink collectSink
	if err := src.ExtractFrom(ctx, &sink, filament.ExtractOpts{Resources: []string{"charges"}, Parallelism: 1}, plan); err != nil {
		t.Fatalf("extract from: %v", err)
	}

	if gotCreatedGte != "1679090539" {
		t.Fatalf("created[gte] = %q, want the stored watermark as a plain epoch integer", gotCreatedGte)
	}
	if !strings.Contains(rawQuery, "created%5Bgte%5D=1679090539") {
		t.Fatalf("raw query = %q, want the bracketed filter percent-encoded", rawQuery)
	}
	if len(sink.records) != 1 {
		t.Fatalf("records = %d, want the single charge past the watermark", len(sink.records))
	}
	// The watermark advances to the newest created seen, not the oldest.
	if got, want := sink.records[0].Key, []string{"1679090700"}; !slices.Equal(got, want) {
		t.Fatalf("record key = %v, want %v", got, want)
	}
}

// TestConnection probes the first top-level, non-streaming resource with only
// the config scope bound, so `customers` has to be declared first and has to
// need no parent capture.
func TestStripeTestConnectionProbesCustomers(t *testing.T) {
	var paths []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"list","url":"/v1/customers","has_more":true,"data":[{"id":"cus_1"}]}`)
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(stripeManifest), "https://api.stripe.com", api.URL, 1))
	src := NewManifest(manifestData)
	if err := src.TestConnection(context.Background(), filament.NewConfig(map[string]any{
		"api_key": "rk_test_123",
	})); err != nil {
		t.Fatalf("test connection: %v", err)
	}
	// has_more is true, but validation must not walk pagination.
	if len(paths) != 1 || paths[0] != "/v1/customers" {
		t.Fatalf("probe requests = %v, want exactly one GET /v1/customers", paths)
	}
}

// The shipped manifest paces at 0.3 req/s to stay under PostHog's 1200/hour
// analytics ceiling, which would add ~3.3s of real sleep to every multi-page
// test. Pagination is what these tests exercise, not throttling, so they lift
// the ceiling the same way other connectors swap in a test base URL. A missed
// replacement only makes the test slow, never wrong.
func unthrottledPostHogManifest(t *testing.T) []byte {
	t.Helper()
	const throttled = "requests_per_second: 0.3"
	if !strings.Contains(string(posthogManifest), throttled) {
		t.Fatalf("manifest no longer contains %q — update the test helper", throttled)
	}
	return []byte(strings.Replace(string(posthogManifest), throttled, "requests_per_second: 1000", 1))
}

func TestNewPostHogSpecAndEmbeddedManifest(t *testing.T) {
	ctx := context.Background()
	src := NewPostHog()
	spec := src.Spec()
	if spec.Name != "posthog" || spec.DisplayName != "PostHog" {
		t.Fatalf("spec identity = %q/%q, want posthog/PostHog", spec.Name, spec.DisplayName)
	}
	fields := map[string]filament.ConfigField{}
	for _, field := range spec.Config.Fields {
		fields[field.Name] = field
	}
	if len(fields) != 3 {
		t.Fatalf("config fields = %#v, want api_key, project_id and host", spec.Config.Fields)
	}
	if fields["api_key"].Type != filament.FieldSecret || !fields["api_key"].Required {
		t.Fatalf("api_key field = %#v, want required secret", fields["api_key"])
	}
	if fields["project_id"].Type != filament.FieldString || !fields["project_id"].Required {
		t.Fatalf("project_id field = %#v, want required string", fields["project_id"])
	}
	// host carries a default so US Cloud works with no input; EU Cloud and
	// self-hosted override it.
	if fields["host"].Type != filament.FieldString || fields["host"].Required {
		t.Fatalf("host field = %#v, want optional string", fields["host"])
	}
	if fields["host"].Default != "https://us.posthog.com" {
		t.Fatalf("host default = %#v, want the US Cloud host", fields["host"].Default)
	}
	if err := src.Validate(filament.NewConfig(map[string]any{"api_key": "phx_test_123"})); err == nil {
		t.Fatal("validate without project_id succeeded")
	}
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"api_key":    "phx_test_123",
		"project_id": "12345",
	})); err != nil {
		t.Fatalf("configure embedded PostHog manifest: %v", err)
	}
	defer src.Teardown(ctx)

	discovered, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	want := []string{
		"persons", "events", "cohorts", "feature_flags", "experiments",
		"insights", "dashboards", "actions", "annotations", "surveys",
		"session_recordings",
	}
	if len(discovered.Resources) != len(want) {
		t.Fatalf("resources = %d, want %d", len(discovered.Resources), len(want))
	}
	for i, name := range want {
		if discovered.Resources[i].Name != name {
			t.Fatalf("resource[%d] = %q, want %q", i, discovered.Resources[i].Name, name)
		}
	}
}

// PostHog is the first manifest whose base_url is templated rather than
// literal, so the host arrives through config instead of a string swap. Every
// path is scoped to one project id, and DRF hands back an absolute next-page
// URL that the engine follows as given.
func TestPostHogPaginatesViaNextURLAndScopesToProject(t *testing.T) {
	ctx := context.Background()
	var authorization string
	var paths, limits, offsets []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects/12345/persons/" {
			http.NotFound(w, r)
			return
		}
		authorization = r.Header.Get("Authorization")
		paths = append(paths, r.URL.Path)
		limits = append(limits, r.URL.Query().Get("limit"))
		offsets = append(offsets, r.URL.Query().Get("offset"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("offset") == "" {
			// PostHog documents this id as "Accepts both numeric ID and UUID",
			// and live instances return the UUID even though the response
			// schema claims integer — so both shapes have to survive.
			fmt.Fprintf(w, `{"count":3,"previous":null,"next":%q,"results":[
				{"id":"0516dfdf-d689-58aa-842d-6c88fd4c2423","uuid":"095be615-a8ad-4c33-8e9c-c7612fbf6c9f","name":"ada@example.com",
				 "distinct_ids":["ada"],"properties":{"plan":"pro"},
				 "created_at":"2026-01-02T03:04:05Z","last_seen_at":"2026-02-02T03:04:05Z"},
				{"id":2,"uuid":"18f0a1c4-31d6-4f7c-9a3f-2b1a0c5d6e7f","name":null,
				 "distinct_ids":["grace"],"properties":{},
				 "created_at":"2026-01-03T03:04:05Z","last_seen_at":null}]}`,
				"http://"+r.Host+"/api/projects/12345/persons/?limit=100&offset=100")
			return
		}
		fmt.Fprint(w, `{"count":3,"previous":null,"next":null,"results":[
			{"id":"2c1d3e4f-5a6b-7c8d-9e0f-1a2b3c4d5e6f","uuid":"2c1d3e4f-5a6b-7c8d-9e0f-1a2b3c4d5e6f","name":"linus@example.com",
			 "distinct_ids":["linus"],"properties":null,
			 "created_at":"2026-01-04T03:04:05Z","last_seen_at":"2026-02-04T03:04:05Z"}]}`)
	}))
	defer api.Close()

	src := NewManifest(unthrottledPostHogManifest(t))
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"api_key":    "phx_test_123",
		"project_id": "12345",
		"host":       api.URL,
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"persons"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract persons: %v", err)
	}

	if authorization != "Bearer phx_test_123" {
		t.Fatalf("Authorization = %q, want the personal API key as a bearer token", authorization)
	}
	if len(paths) != 2 {
		t.Fatalf("requests = %v, want the first page and the server's next page", paths)
	}
	if limits[0] != "100" {
		t.Fatalf("limit = %q, want 100 on the first request", limits[0])
	}
	// Page two comes from the server's absolute next URL, so its offset is
	// PostHog's, never one the manifest computed.
	if offsets[0] != "" || offsets[1] != "100" {
		t.Fatalf("offsets = %v, want the second request to carry the server's offset", offsets)
	}
	if len(sink.records) != 3 {
		t.Fatalf("records = %d, want all three persons across both pages", len(sink.records))
	}
	var first map[string]any
	if err := json.Unmarshal(sink.records[0].Data, &first); err != nil {
		t.Fatalf("decode person: %v", err)
	}
	if first["id"] != "0516dfdf-d689-58aa-842d-6c88fd4c2423" || first["uuid"] != "095be615-a8ad-4c33-8e9c-c7612fbf6c9f" {
		t.Fatalf("person projection = %#v", first)
	}
	// The second person carries a numeric id. Typing the column int64 would
	// abort on the UUID above; string has to hold both, coercing the number.
	var second map[string]any
	if err := json.Unmarshal(sink.records[1].Data, &second); err != nil {
		t.Fatalf("decode person: %v", err)
	}
	if second["id"] != "2" {
		t.Fatalf("numeric person id = %#v, want it coerced into the string column", second["id"])
	}
	// distinct_ids and properties stay json rather than being flattened.
	if _, ok := first["distinct_ids"].([]any); !ok {
		t.Fatalf("distinct_ids = %#v, want a json array", first["distinct_ids"])
	}
}

func TestPostHogEventsIncrementalInjectsAfterMinusOverlap(t *testing.T) {
	ctx := context.Background()
	var gotAfter string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects/12345/events/" {
			http.NotFound(w, r)
			return
		}
		gotAfter = r.URL.Query().Get("after")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"next":null,"results":[
			{"id":"01890a5d-0000-0000-0000-000000000001","event":"$pageview","distinct_id":"ada",
			 "timestamp":"2026-08-01T15:00:00Z","properties":{"$browser":"Chrome"},
			 "person":null,"elements":[],"elements_chain":""}]}`)
	}))
	defer api.Close()

	src := NewManifest(posthogManifest)
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"api_key":    "phx_test_123",
		"project_id": "12345",
		"host":       api.URL,
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	// events and insights each declare their own checkpoint key; the
	// cursor_field fallback would not collide here, but naming them keeps the
	// stored key stable if either cursor field is ever renamed.
	prev := map[string]filament.Checkpoint{
		"events": checkpoint.KeysetCheckpoint{
			Mode:   checkpoint.ModeIncremental,
			Cols:   []string{"events_timestamp"},
			Types:  []string{"timestamptz"},
			Shards: []checkpoint.KeysetShard{{Key: []string{"2026-08-01T12:00:00Z"}}},
		}.ToCheckpoint("events"),
	}
	plan, err := src.PlanIncremental(ctx, []string{"events"}, prev, map[string]filament.ResourceCursorConfig{
		"events": {Field: "timestamp"},
	})
	if err != nil {
		t.Fatalf("plan incremental: %v", err)
	}
	ks, ok := checkpoint.ParseKeyset(plan["events"])
	if !ok {
		t.Fatal("plan did not parse as keyset")
	}
	if got, want := ks.Cols, []string{"events_timestamp"}; !slices.Equal(got, want) {
		t.Fatalf("checkpoint cols = %v, want durable watermark only %v", got, want)
	}
	var sink collectSink
	if err := src.ExtractFrom(ctx, &sink, filament.ExtractOpts{Resources: []string{"events"}, Parallelism: 1}, plan); err != nil {
		t.Fatalf("extract from: %v", err)
	}

	// overlap_seconds: 3600 rewinds the stored watermark an hour, because
	// buffered SDKs deliver events well behind their own timestamps.
	if gotAfter != "2026-08-01T11:00:00Z" {
		t.Fatalf("after = %q, want the stored watermark rewound by the overlap window", gotAfter)
	}
	if len(sink.records) != 1 {
		t.Fatalf("records = %d, want the single event past the watermark", len(sink.records))
	}
}

// Regression test. The watermark advances per record as a page streams, so
// re-applying it to a server-issued next URL would narrow the range out from
// under PostHog's own bounds: on a newest-first feed page two would come back
// empty and the run would commit the newest timestamp having skipped the tail.
// Pages after the first must carry the server's query untouched.
func TestPostHogIncrementalLeavesServerNextURLUntouched(t *testing.T) {
	ctx := context.Background()
	var afters, befores []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects/12345/events/" {
			http.NotFound(w, r)
			return
		}
		afters = append(afters, r.URL.Query().Get("after"))
		befores = append(befores, r.URL.Query().Get("before"))
		w.Header().Set("Content-Type", "application/json")
		// Newest first, as PostHog orders events by -timestamp, and the next
		// URL narrows with `before` rather than an offset.
		if r.URL.Query().Get("before") == "" {
			fmt.Fprintf(w, `{"next":%q,"results":[
				{"id":"evt_1","event":"$pageview","distinct_id":"ada",
				 "timestamp":"2026-08-01T15:00:00Z","properties":{},"person":null,
				 "elements":[],"elements_chain":""}]}`,
				"http://"+r.Host+"/api/projects/12345/events/?limit=100&before=2026-08-01T15%3A00%3A00Z")
			return
		}
		fmt.Fprint(w, `{"next":null,"results":[
			{"id":"evt_0","event":"$pageview","distinct_id":"grace",
			 "timestamp":"2026-07-20T09:00:00Z","properties":{},"person":null,
			 "elements":[],"elements_chain":""}]}`)
	}))
	defer api.Close()

	src := NewManifest(unthrottledPostHogManifest(t))
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"api_key":    "phx_test_123",
		"project_id": "12345",
		"host":       api.URL,
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"events"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract events: %v", err)
	}

	if len(afters) != 2 {
		t.Fatalf("requests = %d, want both pages walked", len(afters))
	}
	// Page one had no stored watermark, and page two must not inherit the one
	// page one's own records just produced.
	if afters[0] != "" || afters[1] != "" {
		t.Fatalf("after = %v, want no watermark injected on either page", afters)
	}
	if befores[1] == "" {
		t.Fatalf("before = %v, want the server's own bound preserved on page two", befores)
	}
	if len(sink.records) != 2 {
		t.Fatalf("records = %d, want both pages of events", len(sink.records))
	}
}
