package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

const responseTokenManifest = `
version: 1
name: export
display_name: Export
description: Generic export fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
config:
  tenant_id: {type: string}
connection:
  base_url: https://example.com
resources:
  - name: items
    path: /export
    primary_key: [id]
    fields:
      id: int64
    response:
      records: $.data
      pagination:
        cursor:
          strict: true
          response: continueFrom
          request: query.from
          more: hasMore
    incremental:
      response_cursor: continueFrom
      start_param: from
      inject_into: query
`

func responseTokenTestSource(t *testing.T, handler http.HandlerFunc) *Source {
	t.Helper()
	api := httptest.NewServer(handler)
	t.Cleanup(api.Close)
	src := NewManifest([]byte(strings.Replace(responseTokenManifest, "base_url: https://example.com", "base_url: "+api.URL, 1)))
	if err := src.Configure(t.Context(), filament.NewConfig(map[string]any{"tenant_id": "42"})); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Teardown(context.Background()) })
	return src
}

func TestResponseTokenExportContinuation(t *testing.T) {
	ctx := context.Background()
	calls := 0
	src := responseTokenTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		expected := []string{"", "z", "b", "a"}
		if calls >= len(expected) {
			t.Fatal("unexpected page")
		}
		if r.URL.Query().Get("from") != expected[calls] {
			t.Errorf("from=%q want %q", r.URL.Query().Get("from"), expected[calls])
		}
		calls++
		switch calls {
		case 1:
			fmt.Fprint(w, `{"data":[],"hasMore":true,"continueFrom":"z"}`)
		case 2:
			fmt.Fprint(w, `{"data":[{"id":1}],"hasMore":true,"continueFrom":"b"}`)
		case 3:
			fmt.Fprint(w, `{"data":[],"hasMore":false,"continueFrom":"a"}`)
		case 4:
			fmt.Fprint(w, `{"data":[],"hasMore":false,"continueFrom":"0"}`)
		}
	})
	plan, err := src.PlanIncremental(ctx, []string{"items"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var sink collectSink
	if err := src.ExtractFrom(ctx, &sink, filament.ExtractOpts{Resources: []string{"items"}}, plan); err != nil {
		t.Fatal(err)
	}
	if got := sink.checkpoints["items"]; !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("checkpoint = %v", got)
	}
	if len(sink.records) != 1 || len(sink.records[0].Key) != 0 {
		t.Fatal("export token leaked into row state")
	}
	cp, _ := checkpoint.ParseKeyset(plan["items"])
	cp.Shards[0].Key = sink.checkpoints["items"]
	plan["items"] = cp.ToCheckpoint("items")
	plan, err = src.PlanIncremental(ctx, []string{"items"}, plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	sink = collectSink{}
	if err := src.ExtractFrom(ctx, &sink, filament.ExtractOpts{Resources: []string{"items"}}, plan); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sink.checkpoints["items"], []string{"0"}) {
		t.Fatal("empty run did not advance")
	}
	if columns, err := src.CursorColumns(ctx, "items"); err != nil || len(columns) != 0 {
		t.Fatalf("opaque cursor exposed as column: %v %v", columns, err)
	}
	policy, err := filament.ResolveWriteVersionPolicy(ctx, src, "items", filament.ResourceCursorConfig{}, filament.ModeIncremental, filament.WriteUpsert)
	if err != nil || policy.Strategy != filament.VersionInsertOrder {
		t.Fatalf("version policy = %v %v", policy, err)
	}
	// A token from a different tenant must not be reused.
	src.connector.creds["tenant_id"] = "43"
	next, err := src.PlanIncremental(ctx, []string{"items"}, plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	reset, _ := checkpoint.ParseKeyset(next["items"])
	if len(reset.Shards[0].Key) != 0 {
		t.Fatal("reused another tenant's token")
	}
}

func TestResponseTokenRejectsInvalidExportEnvelopes(t *testing.T) {
	for _, body := range []string{
		`{"data":[],"hasMore":false}`, `{"data":[],"hasMore":false,"continueFrom":null}`,
		`{"data":[],"continueFrom":"x"}`, `{"data":[],"hasMore":"false","continueFrom":"x"}`,
		`{"hasMore":false,"continueFrom":"x"}`, `{"data":{},"hasMore":false,"continueFrom":"x"}`,
		`{"data":[],"hasMore":true,"continueFrom":"same"}`,
	} {
		t.Run(body, func(t *testing.T) {
			calls := 0
			src := responseTokenTestSource(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls > 2 {
					t.Fatal("cursor cycle")
				}
				fmt.Fprint(w, body)
			})
			plan, err := src.PlanIncremental(context.Background(), []string{"items"}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			var sink collectSink
			if err := src.ExtractFrom(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"items"}}, plan); err == nil {
				t.Fatal("accepted invalid envelope")
			}
			if len(sink.checkpoints) != 0 {
				t.Fatal("failed run emitted terminal token")
			}
		})
	}
}

func TestResponseTokenFailedTokenDoesNotRestartFullRead(t *testing.T) {
	calls := 0
	src := responseTokenTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("from") != "saved" {
			t.Error("discarded saved token")
		}
		w.WriteHeader(400)
		fmt.Fprint(w, `{"error":"invalid continuation"}`)
	})
	plan, err := src.PlanIncremental(context.Background(), []string{"items"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cp, _ := checkpoint.ParseKeyset(plan["items"])
	cp.Shards[0].Key = []string{"saved"}
	plan["items"] = cp.ToCheckpoint("items")
	var sink collectSink
	if err := src.ExtractFrom(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"items"}}, plan); err == nil {
		t.Fatal("request should fail")
	}
	if calls != 1 || len(sink.checkpoints) != 0 {
		t.Fatalf("calls=%d checkpoints=%v", calls, sink.checkpoints)
	}
}

func TestResponseTokenValidation(t *testing.T) {
	for _, replacement := range []string{
		"response_cursor: continueFrom\n      cursor_field: id",
		"response_cursor: continueFrom\n      comparator: lex",
		"response_cursor: continueFrom\n      overlap_seconds: 1",
		"response_cursor: other",
	} {
		if _, err := manifest.Parse([]byte(strings.Replace(responseTokenManifest, "response_cursor: continueFrom", replacement, 1))); err == nil {
			t.Fatalf("accepted %s", replacement)
		}
	}
	src := responseTokenTestSource(t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected request") })
	for _, cfg := range []filament.ResourceCursorConfig{{Field: "id"}, {LookbackSeconds: 1}} {
		if _, err := src.PlanIncremental(t.Context(), []string{"items"}, nil, map[string]filament.ResourceCursorConfig{"items": cfg}); err == nil {
			t.Fatal("accepted row cursor configuration")
		}
	}
}
