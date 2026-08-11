package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/connectors/http/internal/pipeline"
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
	if len(sink.records[0].Key) != 1 || sink.records[0].Key[0] != "two" {
		t.Fatalf("record key = %v, want [two]", sink.records[0].Key)
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
	src := NewManifest("probe", "Probe", data, filament.ConfigSchema{})
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

func TestSourceExtractFromSeedsAndEmitsWatermark(t *testing.T) {
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
	if got, want := ks.Cols, []string{"cursor", "items_since"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
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
	if gotSince != "2026-01-01T00:00:00Z" {
		t.Fatalf("since = %q, want previous watermark", gotSince)
	}
	if len(sink.records) != 1 {
		t.Fatalf("records = %d, want 1", len(sink.records))
	}
	if got, want := sink.records[0].Key, []string{"", "2026-01-02T00:00:00Z"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("record key = %v, want %v", got, want)
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

func TestToIngestionRecordWrapsHTTPAPIPayload(t *testing.T) {
	rec := toIngestionRecord(pipeline.Record{
		Resource: "databases",
		KeyJSON:  []byte(`{"id":"db1"}`),
		DataJSON: []byte(`{"id":"db1","title":[{"plain_text":"Team Tasks"}]}`),
	})
	var got map[string]json.RawMessage
	if err := json.Unmarshal(rec.Data, &got); err != nil {
		t.Fatalf("record data is not json: %v", err)
	}
	if string(got["id"]) != `"db1"` {
		t.Fatalf("id = %s, want db1", got["id"])
	}
	var payload map[string]any
	if err := json.Unmarshal(got["data"], &payload); err != nil {
		t.Fatalf("data payload is not json: %v", err)
	}
	if payload["id"] != "db1" {
		t.Fatalf("payload id = %v, want db1", payload["id"])
	}
	if rec.ID != "db1" {
		t.Fatalf("record id = %q, want db1", rec.ID)
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
	var authorization string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
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
	src := NewManifest("attio", "Attio", manifestData, filament.ConfigSchema{})
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "attio-key"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"objects", "lists"}}); err != nil {
		t.Fatalf("extract Attio catalog: %v", err)
	}
	if authorization != "Bearer attio-key" {
		t.Fatalf("Authorization = %q, want Bearer attio-key", authorization)
	}
	if len(sink.records) != 2 {
		t.Fatalf("records = %#v, want one Attio object and list", sink.records)
	}
}

func TestSlackEmbeddedManifestAndMessageFanOut(t *testing.T) {
	ctx := context.Background()
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
	src := NewManifest("slack", "Slack", manifestData, filament.ConfigSchema{})
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"token": "xoxb-test"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

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
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"children"}}); err != nil {
		t.Fatalf("extract children: %v", err)
	}
	if parentRequests != 1 || childRequests != 1 {
		t.Fatalf("requests parent=%d child=%d, want 1 each", parentRequests, childRequests)
	}
	if len(sink.records) != 1 || sink.records[0].Resource != "children" {
		t.Fatalf("emitted records = %#v, want only selected child", sink.records)
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

type collectSink struct {
	records []filament.Record
}

func (s *collectSink) Push(r filament.Record) error {
	s.records = append(s.records, r)
	return nil
}

func (s *collectSink) PushBatch(records []filament.Record) error {
	for _, r := range records {
		if err := s.Push(r); err != nil {
			return err
		}
	}
	return nil
}

func writeTestManifest(t *testing.T, baseURL string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	data := fmt.Sprintf(`version: 1
name: test_http
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
	src := NewManifest("resend", "Resend", manifestData, filament.ConfigSchema{})
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
	src := NewManifest("resend", "Resend", manifestData, filament.ConfigSchema{})
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
	src := NewManifest("resend", "Resend", manifestData, filament.ConfigSchema{})
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
	src := NewManifest("resend", "Resend", manifestData, filament.ConfigSchema{})
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
	settingsSrc := NewManifest("resend", "Resend", manifestData, filament.ConfigSchema{})
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
	src := NewManifest("resend", "Resend", manifestData, filament.ConfigSchema{})
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
		if data["sent_at"] != "2026-08-22T15:33:12.000Z" {
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
	src := NewManifest("resend", "Resend", manifestData, filament.ConfigSchema{})
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
