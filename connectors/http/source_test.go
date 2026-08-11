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
		{
			name: "stripe", source: NewStripe(),
			description: "Payments platform for online businesses to accept payments, run billing, and manage revenue.",
			darkLogo:    "https://cdn.getgalaxy.io/sources/source-icon-stripe-dark.svg",
			lightLogo:   "https://cdn.getgalaxy.io/sources/source-icon-stripe-light.svg",
		},
		{
			name: "hubspot", source: NewHubSpot(),
			description: "CRM and marketing platform covering contacts, deals, tickets, campaigns, content, and conversations.",
			darkLogo:    "https://cdn.getgalaxy.io/sources/source-icon-hubspot-dark.svg",
			lightLogo:   "https://cdn.getgalaxy.io/sources/source-icon-hubspot-light.svg",
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
	if len(fields) != 2 {
		t.Fatalf("config fields = %#v, want api_key and api_version", spec.Config.Fields)
	}
	if fields["api_key"].Type != filament.FieldSecret || !fields["api_key"].Required {
		t.Fatalf("api_key field = %#v, want required secret", fields["api_key"])
	}
	if fields["api_version"].Type != filament.FieldString || fields["api_version"].Required {
		t.Fatalf("api_version field = %#v, want optional string", fields["api_version"])
	}
	if fields["api_version"].Default == nil || fields["api_version"].Default == "" {
		t.Fatalf("api_version default = %#v, want a pinned Stripe version", fields["api_version"].Default)
	}
	if err := src.Validate(filament.NewConfig(map[string]any{})); err == nil {
		t.Fatal("validate without API key succeeded")
	}
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"api_key": "sk_test_123",
	})); err != nil {
		t.Fatalf("configure embedded Stripe manifest: %v", err)
	}
	defer src.Teardown(ctx)

	discovered, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	want := []string{
		"account", "accounts", "balance_transactions", "charges", "payment_intents",
		"setup_intents", "payment_methods", "refunds", "disputes", "payouts", "transfers",
		"application_fees", "topups", "customers", "invoices", "invoice_line_items",
		"invoiceitems", "invoice_payments", "credit_notes", "subscriptions",
		"subscription_items", "subscription_schedules", "products", "prices", "plans",
		"coupons", "promotion_codes", "tax_rates", "tax_ids", "shipping_rates",
		"checkout_sessions", "checkout_session_line_items", "payment_links", "quotes",
		"events", "reviews", "early_fraud_warnings", "files", "file_links",
		"billing_meters", "webhook_endpoints",
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

// Stripe returns no next-page token, so the paginator has to read the id off
// the last element of `data` and echo it back as starting_after.
func TestStripePaginatesOnLastObjectIDAndAuthenticatesWithKeyAsUser(t *testing.T) {
	ctx := context.Background()
	var user, pass, version string
	var startingAfter, limits []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ = r.BasicAuth()
		version = r.Header.Get("Stripe-Version")
		startingAfter = append(startingAfter, r.URL.Query().Get("starting_after"))
		limits = append(limits, r.URL.Query().Get("limit"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/v1/charges" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("starting_after") == "" {
			fmt.Fprint(w, `{"object":"list","url":"/v1/charges","has_more":true,"data":[
				{"id":"ch_1","object":"charge","amount":1000,"currency":"usd","created":1700000000,"status":"succeeded"},
				{"id":"ch_2","object":"charge","amount":2000,"currency":"usd","created":1700000100,"status":"succeeded"}]}`)
			return
		}
		fmt.Fprint(w, `{"object":"list","url":"/v1/charges","has_more":false,"data":[
			{"id":"ch_3","object":"charge","amount":3000,"currency":"usd","created":1700000200,"status":"succeeded"}]}`)
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(stripeManifest), "https://api.stripe.com", api.URL, 1))
	src := NewManifest("stripe", "Stripe", manifestData, filament.ConfigSchema{})
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "sk_test_123"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"charges"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract charges: %v", err)
	}

	if user != "sk_test_123" || pass != "" {
		t.Fatalf("basic auth = %q/%q, want the API key as user with no password", user, pass)
	}
	if version == "" {
		t.Fatal("Stripe-Version header was not sent")
	}
	if len(startingAfter) != 2 || startingAfter[0] != "" || startingAfter[1] != "ch_2" {
		t.Fatalf("starting_after = %v, want the last id of page one on the second request", startingAfter)
	}
	for i, limit := range limits {
		if limit != "100" {
			t.Fatalf("limit[%d] = %q, want Stripe's 100 maximum", i, limit)
		}
	}
	if len(sink.records) != 3 {
		t.Fatalf("records = %d, want all three charges across both pages", len(sink.records))
	}
	var first map[string]any
	if err := json.Unmarshal(sink.records[0].Data, &first); err != nil {
		t.Fatalf("decode charge: %v", err)
	}
	if first["id"] != "ch_1" || first["amount"] != float64(1000) || first["status"] != "succeeded" {
		t.Fatalf("charge projection = %#v", first)
	}
	if _, ok := first["raw"].(map[string]any); !ok {
		t.Fatalf("raw remainder = %#v, want the unmapped payload", first["raw"])
	}
}

// /v1/account is the one Stripe resource that returns a bare object instead of
// a list, and the one that rejects `limit` as an unknown parameter — so it
// carries neither the shared page size nor pagination.
func TestStripeAccountSingletonSendsNoListParams(t *testing.T) {
	ctx := context.Background()
	var query string
	var requests int
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"acct_1","object":"account","country":"US","default_currency":"usd",
			"charges_enabled":true,"payouts_enabled":true,"business_profile":{"name":"Galaxy"}}`)
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(stripeManifest), "https://api.stripe.com", api.URL, 1))
	src := NewManifest("stripe", "Stripe", manifestData, filament.ConfigSchema{})
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "sk_test_123"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"account"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract account: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want a single unpaginated fetch", requests)
	}
	if query != "" {
		t.Fatalf("query = %q, want no list parameters on /v1/account", query)
	}
	if len(sink.records) != 1 {
		t.Fatalf("records = %d, want the account object as one row", len(sink.records))
	}
	var data map[string]any
	if err := json.Unmarshal(sink.records[0].Data, &data); err != nil {
		t.Fatalf("decode account: %v", err)
	}
	if data["id"] != "acct_1" || data["country"] != "US" || data["charges_enabled"] != true {
		t.Fatalf("account projection = %#v", data)
	}
}

func TestStripeInvoiceLineItemsFanOutFromInvoices(t *testing.T) {
	ctx := context.Background()
	var linePaths []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/invoices":
			fmt.Fprint(w, `{"object":"list","has_more":false,"data":[
				{"id":"in_1","object":"invoice","status":"paid","total":1000,"currency":"usd","created":1700000000},
				{"id":"in_2","object":"invoice","status":"open","total":2000,"currency":"usd","created":1700000100}]}`)
		case "/v1/invoices/in_1/lines", "/v1/invoices/in_2/lines":
			linePaths = append(linePaths, r.URL.Path)
			fmt.Fprintf(w, `{"object":"list","has_more":false,"data":[
				{"id":"il_%s","object":"line_item","amount":1000,"currency":"usd","description":"Seat"}]}`,
				strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/invoices/"), "/lines"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	manifestData := []byte(strings.Replace(string(stripeManifest), "https://api.stripe.com", api.URL, 1))
	src := NewManifest("stripe", "Stripe", manifestData, filament.ConfigSchema{})
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_key": "sk_test_123"})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"invoice_line_items"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract invoice line items: %v", err)
	}
	if len(linePaths) != 2 {
		t.Fatalf("line requests = %v, want one per invoice", linePaths)
	}
	if len(sink.records) != 2 {
		t.Fatalf("records = %d, want two line items and no invoices", len(sink.records))
	}
	seen := map[string]bool{}
	for _, rec := range sink.records {
		if rec.Resource != "invoice_line_items" {
			t.Fatalf("resource = %q, want invoice_line_items only", rec.Resource)
		}
		var data map[string]any
		if err := json.Unmarshal(rec.Data, &data); err != nil {
			t.Fatalf("decode line item: %v", err)
		}
		seen[data["invoice_id"].(string)] = true
	}
	if !seen["in_1"] || !seen["in_2"] {
		t.Fatalf("invoice_id values = %v, want both parents denormalized onto the child", seen)
	}
}

// The events watermark is a unix timestamp injected through Stripe's bracketed
// range filter, and it must stay pinned to the value the run started with:
// Stripe lists newest-first, so a floor that advanced with the running max
// would filter page two down to nothing and truncate the run.
func TestStripeEventsIncrementalFloorIsFrozenAcrossPages(t *testing.T) {
	ctx := context.Background()
	var floors []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		floors = append(floors, r.URL.Query().Get("created[gte]"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("starting_after") == "" {
			fmt.Fprint(w, `{"object":"list","has_more":true,"data":[
				{"id":"evt_2","object":"event","type":"charge.succeeded","created":1700000200}]}`)
			return
		}
		fmt.Fprint(w, `{"object":"list","has_more":false,"data":[
			{"id":"evt_1","object":"event","type":"charge.succeeded","created":1700000100}]}`)
	}))
	defer api.Close()

	path := filepath.Join(t.TempDir(), "stripe.yaml")
	manifestData := strings.Replace(string(stripeManifest), "https://api.stripe.com", api.URL, 1)
	if err := os.WriteFile(path, []byte(manifestData), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"manifest_path": path,
		"api_key":       "sk_test_123",
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	prev := map[string]filament.Checkpoint{
		"events": checkpoint.KeysetCheckpoint{
			Cols:   []string{"cursor", "events_created"},
			Types:  []string{"string", "string"},
			Shards: []checkpoint.KeysetShard{{Key: []string{"", "1700000000"}}},
		}.ToCheckpoint("events"),
	}
	var sink collectSink
	if err := src.ExtractFrom(ctx, &sink, filament.ExtractOpts{Resources: []string{"events"}, Parallelism: 1}, prev); err != nil {
		t.Fatalf("extract from: %v", err)
	}
	if len(floors) != 2 {
		t.Fatalf("requests = %v, want both pages fetched", floors)
	}
	for i, floor := range floors {
		if floor != "1700000000" {
			t.Fatalf("created[gte] on page %d = %q, want the checkpointed floor", i+1, floor)
		}
	}
	if len(sink.records) != 2 {
		t.Fatalf("records = %d, want both pages of events", len(sink.records))
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

func TestNewHubSpotSpecAndEmbeddedManifest(t *testing.T) {
	ctx := context.Background()
	src := NewHubSpot()
	spec := src.Spec()
	if spec.Name != "hubspot" || spec.DisplayName != "HubSpot" {
		t.Fatalf("spec identity = %q/%q, want hubspot/HubSpot", spec.Name, spec.DisplayName)
	}
	fields := map[string]filament.ConfigField{}
	for _, field := range spec.Config.Fields {
		fields[field.Name] = field
	}
	if fields["access_token"].Type != filament.FieldSecret || !fields["access_token"].Required {
		t.Fatalf("access_token field = %#v, want required secret", fields["access_token"])
	}
	// Every property selector is an optional string that ships a non-empty
	// default: HubSpot's list endpoints return only three properties unless
	// the request names the ones it wants.
	for _, name := range []string{
		"contact_properties", "company_properties", "deal_properties", "ticket_properties",
		"lead_properties", "engagement_properties", "product_properties", "quote_properties",
		"commerce_properties", "object_properties",
	} {
		field, ok := fields[name]
		if !ok {
			t.Fatalf("config field %q missing", name)
		}
		if field.Type != filament.FieldString || field.Required {
			t.Fatalf("%s field = %#v, want optional string", name, field)
		}
		if field.Default == nil || field.Default == "" {
			t.Fatalf("%s default = %#v, want a non-empty property list", name, field.Default)
		}
	}
	if err := src.Validate(filament.NewConfig(map[string]any{})); err == nil {
		t.Fatal("validate without access token succeeded")
	}
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"access_token": "pat-na1-test",
	})); err != nil {
		t.Fatalf("configure embedded HubSpot manifest: %v", err)
	}
	defer src.Teardown(ctx)

	discovered, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	want := []string{
		"contacts", "companies", "deals", "tickets", "leads", "line_items", "products",
		"quotes", "feedback_submissions", "goal_targets", "calls", "emails", "meetings",
		"notes", "tasks", "communications", "postal_mail", "carts", "orders", "invoices",
		"commerce_payments", "subscriptions", "discounts", "fees", "taxes", "appointments",
		"courses", "listings", "services", "owners", "object_schemas",
		"contact_property_definitions", "company_property_definitions",
		"deal_property_definitions", "ticket_property_definitions", "deal_pipelines",
		"ticket_pipelines", "deal_pipeline_stages", "ticket_pipeline_stages", "lists",
		"list_memberships", "forms", "marketing_emails", "campaigns", "marketing_events",
		"subscription_definitions", "blog_posts", "blog_authors", "blog_tags", "site_pages",
		"landing_pages", "url_redirects", "domains", "hubdb_tables", "hubdb_rows", "files",
		"file_folders", "conversation_inboxes", "conversation_channels",
		"conversation_channel_accounts", "conversation_threads", "conversation_messages",
		"account_details", "users", "teams", "roles", "currency_exchange_rates", "tax_rates",
		"audit_logs", "sequences", "meeting_links", "event_definitions",
	}
	if len(discovered.Resources) != len(want) {
		t.Fatalf("resources = %d, want %d", len(discovered.Resources), len(want))
	}
	for i, name := range want {
		if discovered.Resources[i].Name != name || discovered.Resources[i].Selector != name {
			t.Fatalf("resource[%d] = %#v, want %q", i, discovered.Resources[i], name)
		}
	}
}

// HubSpot pages every collection on paging.next.after and drops the whole
// `paging` object on the final page, so the cursor paginator has to treat a
// missing path as termination rather than an error.
func TestHubSpotContactsPaginateOnPagingNextAfter(t *testing.T) {
	ctx := context.Background()
	var authorization string
	var afters, properties, associations, limits []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/crm/v3/objects/contacts" {
			http.NotFound(w, r)
			return
		}
		after := r.URL.Query().Get("after")
		afters = append(afters, after)
		properties = append(properties, r.URL.Query().Get("properties"))
		associations = append(associations, r.URL.Query().Get("associations"))
		limits = append(limits, r.URL.Query().Get("limit"))
		if after == "" {
			fmt.Fprint(w, `{"results":[{"id":"1","properties":{"email":"a@example.com"},`+
				`"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z","archived":false}],`+
				`"paging":{"next":{"after":"page-2","link":"https://api.hubapi.com/x"}}}`)
			return
		}
		fmt.Fprint(w, `{"results":[{"id":"2","properties":{"email":"b@example.com"},`+
			`"createdAt":"2026-01-03T00:00:00Z","updatedAt":"2026-01-04T00:00:00Z","archived":false,`+
			`"associations":{"companies":{"results":[{"id":"9","type":"contact_to_company"}]}}}]}`)
	}))
	defer api.Close()

	src := newHubSpotTestSource(t, api.URL)
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"contacts"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract contacts: %v", err)
	}
	if authorization != "Bearer pat-na1-test" {
		t.Fatalf("Authorization = %q, want Bearer pat-na1-test", authorization)
	}
	if len(afters) != 2 || afters[0] != "" || afters[1] != "page-2" {
		t.Fatalf("after params = %v, want [\"\", \"page-2\"]", afters)
	}
	if limits[0] != "100" {
		t.Fatalf("limit = %q, want 100 (HubSpot's per-page maximum)", limits[0])
	}
	if !strings.Contains(properties[0], "lifecyclestage") {
		t.Fatalf("properties = %q, want the configured contact property list", properties[0])
	}
	if associations[0] != "companies,deals,tickets" {
		t.Fatalf("associations = %q, want companies,deals,tickets", associations[0])
	}
	if len(sink.records) != 2 {
		t.Fatalf("records = %d, want 2 contacts across both pages", len(sink.records))
	}
	var first map[string]any
	if err := json.Unmarshal(sink.records[0].Data, &first); err != nil {
		t.Fatalf("decode contact: %v", err)
	}
	if first["id"] != "1" || first["updated_at"] == nil {
		t.Fatalf("contact = %#v, want id 1 with a projected updated_at", first)
	}
	props, ok := first["properties"].(map[string]any)
	if !ok || props["email"] != "a@example.com" {
		t.Fatalf("properties = %#v, want the raw HubSpot property map", first["properties"])
	}
}

// deal_pipeline_stages fans out one request per pipeline captured from
// deal_pipelines, and lists pages by mutating the POST body rather than the
// query string.
func TestHubSpotPipelineFanOutAndListSearchBodyPagination(t *testing.T) {
	ctx := context.Background()
	var stagePaths []string
	var listBodies []map[string]any
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/crm/v3/pipelines/deals":
			fmt.Fprint(w, `{"results":[{"id":"default","label":"Sales Pipeline","displayOrder":0,`+
				`"archived":false,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z",`+
				`"stages":[]},{"id":"p2","label":"Renewals","displayOrder":1,"archived":false,`+
				`"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","stages":[]}]}`)
		case strings.HasPrefix(r.URL.Path, "/crm/v3/pipelines/deals/") && strings.HasSuffix(r.URL.Path, "/stages"):
			stagePaths = append(stagePaths, r.URL.Path)
			fmt.Fprint(w, `{"results":[{"id":"s1","label":"Appointment scheduled","displayOrder":0,`+
				`"metadata":{"isClosed":"false","probability":"0.2"},"archived":false,`+
				`"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}]}`)
		case r.URL.Path == "/crm/v3/lists/search":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode list search body: %v", err)
			}
			listBodies = append(listBodies, body)
			if len(listBodies) == 1 {
				// A full page keeps the offset paginator going.
				items := make([]string, 0, 500)
				for i := 0; i < 500; i++ {
					items = append(items, fmt.Sprintf(`{"listId":"%d","name":"list-%d","objectTypeId":"0-1",`+
						`"processingType":"DYNAMIC","processingStatus":"COMPLETE","listVersion":1}`, i, i))
				}
				fmt.Fprintf(w, `{"total":501,"offset":500,"hasMore":true,"lists":[%s]}`, strings.Join(items, ","))
				return
			}
			fmt.Fprint(w, `{"total":501,"offset":501,"hasMore":false,"lists":[{"listId":"500",`+
				`"name":"last","objectTypeId":"0-1","processingType":"MANUAL","processingStatus":"COMPLETE",`+
				`"listVersion":2}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	src := newHubSpotTestSource(t, api.URL)
	defer src.Teardown(ctx)

	var stages collectSink
	if err := src.Extract(ctx, &stages, filament.ExtractOpts{Resources: []string{"deal_pipeline_stages"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract deal pipeline stages: %v", err)
	}
	got := map[string]bool{}
	for _, path := range stagePaths {
		got[path] = true
	}
	if len(stagePaths) != 2 ||
		!got["/crm/v3/pipelines/deals/default/stages"] ||
		!got["/crm/v3/pipelines/deals/p2/stages"] {
		t.Fatalf("stage paths = %v, want one request per captured pipeline id", stagePaths)
	}
	if len(stages.records) != 2 {
		t.Fatalf("stage records = %d, want one per pipeline (parents must not emit)", len(stages.records))
	}
	for _, rec := range stages.records {
		if rec.Resource != "deal_pipeline_stages" {
			t.Fatalf("resource = %q, want deal_pipeline_stages", rec.Resource)
		}
		var data map[string]any
		if err := json.Unmarshal(rec.Data, &data); err != nil {
			t.Fatalf("decode stage: %v", err)
		}
		if data["pipeline_id"] != "default" && data["pipeline_id"] != "p2" {
			t.Fatalf("pipeline_id = %#v, want the captured parent id", data["pipeline_id"])
		}
	}

	var lists collectSink
	if err := src.Extract(ctx, &lists, filament.ExtractOpts{Resources: []string{"lists"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract lists: %v", err)
	}
	if len(listBodies) != 2 {
		t.Fatalf("list search requests = %d, want 2", len(listBodies))
	}
	if listBodies[0]["offset"] != float64(0) || listBodies[0]["count"] != float64(500) {
		t.Fatalf("first list body = %#v, want offset 0 count 500", listBodies[0])
	}
	if listBodies[1]["offset"] != float64(500) {
		t.Fatalf("second list body = %#v, want offset advanced to 500", listBodies[1])
	}
	if len(lists.records) != 501 {
		t.Fatalf("list records = %d, want 501 across both pages", len(lists.records))
	}
}

// Incremental streams inject their watermark as the API's own since-parameter.
// The first run must not send one at all — HubSpot has no "beginning of time"
// sentinel, so an empty updatedAfter would be a 400.
func TestHubSpotBlogPostsInjectUpdatedAfterFromCheckpoint(t *testing.T) {
	ctx := context.Background()
	var updatedAfters []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/cms/v3/blogs/posts" {
			http.NotFound(w, r)
			return
		}
		if _, ok := r.URL.Query()["updatedAfter"]; !ok {
			updatedAfters = append(updatedAfters, "<absent>")
		} else {
			updatedAfters = append(updatedAfters, r.URL.Query().Get("updatedAfter"))
		}
		fmt.Fprint(w, `{"total":1,"results":[{"id":"77","name":"Launch","slug":"launch",`+
			`"created":"2026-02-01T00:00:00Z","updated":"2026-02-05T12:00:00Z","state":"PUBLISHED"}]}`)
	}))
	defer api.Close()

	src := newHubSpotTestSource(t, api.URL)
	defer src.Teardown(ctx)

	plan, err := src.PlanResume(ctx, []string{"blog_posts"}, nil)
	if err != nil {
		t.Fatalf("plan resume: %v", err)
	}
	ks, ok := checkpoint.ParseKeyset(plan["blog_posts"])
	if !ok {
		t.Fatal("blog_posts plan did not parse as keyset")
	}
	if len(ks.Cols) != 2 || ks.Cols[0] != "cursor" || ks.Cols[1] != "blog_posts_updated_after" {
		t.Fatalf("checkpoint cols = %v, want [cursor blog_posts_updated_after]", ks.Cols)
	}

	var first collectSink
	if err := src.Extract(ctx, &first, filament.ExtractOpts{Resources: []string{"blog_posts"}, Parallelism: 1}); err != nil {
		t.Fatalf("extract blog posts: %v", err)
	}
	if len(first.records) != 1 {
		t.Fatalf("records = %d, want 1", len(first.records))
	}
	if got := first.records[0].Key; len(got) != 2 || got[1] != "2026-02-05T12:00:00Z" {
		t.Fatalf("record key = %v, want the max `updated` value as the watermark", got)
	}

	prev := map[string]filament.Checkpoint{
		"blog_posts": checkpoint.KeysetCheckpoint{
			Cols:   []string{"cursor", "blog_posts_updated_after"},
			Types:  []string{"string", "string"},
			Shards: []checkpoint.KeysetShard{{Key: []string{"", "2026-02-05T12:00:00Z"}}},
		}.ToCheckpoint("blog_posts"),
	}
	var second collectSink
	if err := src.ExtractFrom(ctx, &second, filament.ExtractOpts{Resources: []string{"blog_posts"}, Parallelism: 1}, prev); err != nil {
		t.Fatalf("extract blog posts from checkpoint: %v", err)
	}
	if len(updatedAfters) != 2 {
		t.Fatalf("requests = %v, want two", updatedAfters)
	}
	if updatedAfters[0] != "<absent>" {
		t.Fatalf("first run sent updatedAfter=%q, want no parameter at all", updatedAfters[0])
	}
	// overlap_seconds: 60 rewinds the floor so retroactive edits are re-read.
	if updatedAfters[1] != "2026-02-05T11:59:00Z" {
		t.Fatalf("second run updatedAfter = %q, want the watermark minus the 60s overlap", updatedAfters[1])
	}
}

func newHubSpotTestSource(t *testing.T, baseURL string) *Source {
	t.Helper()
	manifestData := []byte(strings.Replace(string(hubspotManifest), "https://api.hubapi.com", baseURL, 1))
	src := NewManifest("hubspot", "HubSpot", manifestData, filament.ConfigSchema{})
	if err := src.Configure(context.Background(), filament.NewConfig(map[string]any{
		"access_token": "pat-na1-test",
	})); err != nil {
		t.Fatalf("configure HubSpot test manifest: %v", err)
	}
	return src
}
