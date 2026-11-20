package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
)

// writeAfterManifest declares invoices after invoice_items, two top-level
// resources the engine would otherwise read at the same time.
func writeAfterManifest(t *testing.T, baseURL string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest.yaml")
	data := fmt.Sprintf(`version: 1
name: ordered
display_name: Ordered
description: Read order fixture.
dark_logo_url: https://cdn.example.com/dark.svg
light_logo_url: https://cdn.example.com/light.svg
connection:
  base_url: %s
defaults:
  response:
    records: $.data
resources:
  - name: invoices
    path: /invoices
    primary_key: [id]
    after: [invoice_items]
  - name: invoice_items
    path: /invoice-items
    primary_key: [id]
    response:
      pagination:
        cursor:
          response: next
          request: query.from
          more: more
`, baseURL)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func TestExtractReadsResourceAfterItsDependencies(t *testing.T) {
	ctx := context.Background()
	var itemsDone, invoicesEarly atomic.Bool
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/invoice-items":
			// Hold the items read open long enough that a concurrent invoices
			// read would arrive before it ends.
			time.Sleep(100 * time.Millisecond)
			if r.URL.Query().Get("from") == "" {
				fmt.Fprint(w, `{"data":[{"id":1}],"next":"p2","more":true}`)
				return
			}
			itemsDone.Store(true)
			fmt.Fprint(w, `{"data":[{"id":2}],"more":false}`)
		case "/invoices":
			if !itemsDone.Load() {
				invoicesEarly.Store(true)
			}
			fmt.Fprint(w, `{"data":[{"id":10}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"manifest_path": writeAfterManifest(t, api.URL),
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	var sink collectSink
	if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"invoices", "invoice_items"}}); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if invoicesEarly.Load() {
		t.Fatal("invoices were read before invoice_items finished")
	}
	if len(sink.records) != 3 {
		t.Fatalf("records = %d, want 3", len(sink.records))
	}
}

func TestExtractIgnoresUnselectedDependency(t *testing.T) {
	ctx := context.Background()
	var itemsRequested atomic.Bool
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/invoice-items" {
			itemsRequested.Store(true)
		}
		fmt.Fprint(w, `{"data":[{"id":10}]}`)
	}))
	defer api.Close()

	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"manifest_path": writeAfterManifest(t, api.URL),
	})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer src.Teardown(ctx)

	done := make(chan error, 1)
	var sink collectSink
	go func() { done <- src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"invoices"}}) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("extract: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("invoices blocked on an unselected dependency")
	}
	if itemsRequested.Load() {
		t.Fatal("an unselected dependency was read")
	}
	if len(sink.records) != 1 {
		t.Fatalf("records = %d, want 1", len(sink.records))
	}
}
