package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
)

func rechargeTestSource(t *testing.T, handler http.HandlerFunc) *Source {
	t.Helper()
	api := httptest.NewServer(handler)
	t.Cleanup(api.Close)
	data := strings.Replace(string(rechargeManifest), "https://api.rechargeapps.com", api.URL, 1)
	data = strings.Replace(data, "requests_per_second: 2", "requests_per_second: 1000", 1)
	src := NewManifest([]byte(data))
	if err := src.Configure(t.Context(), filament.NewConfig(map[string]any{"api_token": "rc_test"})); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Teardown(t.Context()) })
	return src
}

func TestNewRechargeSpecAndEmbeddedManifest(t *testing.T) {
	ctx := t.Context()
	src := NewRecharge()
	spec := src.Spec()
	if spec.Name != "recharge" || spec.DisplayName != "Recharge" || len(spec.Config.Fields) != 1 {
		t.Fatalf("spec = %#v", spec)
	}
	if f := spec.Config.Fields[0]; f.Name != "api_token" || f.Type != filament.FieldSecret || !f.Required {
		t.Fatalf("api_token field = %#v", f)
	}
	if err := src.Validate(filament.NewConfig(nil)); err == nil {
		t.Fatal("validate without token succeeded")
	}
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"api_token": "rc_test"})); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(ctx)
	discovered, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var names, enabled []string
	for _, r := range discovered.Resources {
		names = append(names, r.Name)
		if r.Metadata["default_resources"] == "true" {
			enabled = append(enabled, r.Name)
		}
	}
	wantEnabled := []string{"store", "customers", "addresses", "subscriptions", "charges", "orders", "onetimes", "discounts", "plans"}
	if len(names) != 19 || !slices.Equal(enabled, wantEnabled) {
		t.Fatalf("resources = %v, defaults = %v", names, enabled)
	}
	incremental := append(slices.Clone(wantEnabled[1:]), "events")
	for _, name := range names {
		cols, err := src.CursorColumns(ctx, name)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(cols) == 1; got != slices.Contains(incremental, name) {
			t.Fatalf("%s cursor columns = %#v", name, cols)
		}
	}
	schema, err := src.Schema(ctx, "products")
	if err != nil || !slices.Equal(schema.PrimaryKey, []string{"external_product_id"}) {
		t.Fatalf("products key = %v, %v", schema.PrimaryKey, err)
	}
}

func TestRechargeReadsStoreSingletonAndCursorPages(t *testing.T) {
	var mu sync.Mutex
	var requests []string
	src := rechargeTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.URL.Path+"?"+r.URL.RawQuery)
		mu.Unlock()
		if r.Header.Get("X-Recharge-Access-Token") != "rc_test" || r.Header.Get("X-Recharge-Version") != "2021-11" {
			t.Errorf("headers = %v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		q := r.URL.Query()
		switch r.URL.Path {
		case "/store":
			fmt.Fprint(w, `{"store":{"id":4797,"name":"store-name","currency":"USD","external_platform":"shopify","default_api_version":"2021-01","timezone":{"iana_name":"America/New_York"},"created_at":"2020-04-22T00:20:52+00:00","updated_at":"2020-04-25T00:20:52+00:00","weight_unit":"g"}}`)
		case "/subscriptions":
			if q.Get("cursor") == "" {
				fmt.Fprint(w, `{"next_cursor":"eyJzdGFydGluZ19iZWZvcmVfaWQiOjF9","previous_cursor":null,"subscriptions":[{"id":9007199254740993,"customer_id":1,"address_id":2,"status":"active","price":"12.00","quantity":2,"external_product_id":{"ecommerce":"4567"},"next_charge_scheduled_at":"2026-02-01T00:00:00","created_at":"2025-01-01T10:00:00+00:00","updated_at":"2026-01-05T09:30:00+00:00","sku_override":false}]}`)
				return
			}
			if q.Get("cursor") != "eyJzdGFydGluZ19iZWZvcmVfaWQiOjF9" || len(q) != 2 || q.Get("limit") != "250" {
				t.Errorf("continuation query = %v", q)
			}
			fmt.Fprint(w, `{"next_cursor":null,"previous_cursor":"prev","subscriptions":[{"id":2,"customer_id":1,"address_id":2,"status":"cancelled","price":"8.50","quantity":1,"cancelled_at":"2026-01-02T00:00:00+00:00","created_at":"2025-01-01T10:00:00+00:00","updated_at":"2026-01-02T00:00:00+00:00"}]}`)
		case "/onetimes":
			if q.Get("include_cancelled") != "true" || q.Get("limit") != "250" {
				t.Errorf("onetime query = %v", q)
			}
			fmt.Fprint(w, `{"next_cursor":null,"previous_cursor":null,"onetimes":[]}`)
		case "/metafields":
			if q.Get("owner_resource") != "subscription" {
				t.Errorf("metafield query = %v", q)
			}
			fmt.Fprint(w, `{"next_cursor":null,"previous_cursor":null,"metafields":[{"id":7,"owner_resource":"subscription","owner_id":"9007199254740993","namespace":"custom","key":"gift","value":"true","value_type":"string","created_at":"2025-01-01T10:00:00+00:00","updated_at":"2025-01-01T10:00:00+00:00"}]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	selected := []string{"store", "subscriptions", "onetimes", "subscription_metafields"}
	var sink collectSink
	if err := src.Extract(t.Context(), &sink, filament.ExtractOpts{Resources: selected}); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, rec := range sink.records {
		counts[rec.Resource]++
		if !slices.Contains(selected, rec.Resource) {
			t.Fatalf("unselected resource emitted: %s", rec.Resource)
		}
		dec := json.NewDecoder(strings.NewReader(string(rec.Data)))
		dec.UseNumber()
		var row map[string]any
		if err := dec.Decode(&row); err != nil {
			t.Fatal(err)
		}
		raw, _ := row["raw"].(map[string]any)
		switch rec.Resource {
		case "store":
			if row["timezone"].(map[string]any)["iana_name"] != "America/New_York" || row["weight_unit"] != "g" {
				t.Fatalf("store projection = %#v", row)
			}
		case "subscriptions":
			if id := row["id"].(json.Number).String(); id == "9007199254740993" {
				if fmt.Sprint(row["price"]) != "12.00" || row["external_product_id"].(map[string]any)["ecommerce"] != "4567" || row["next_charge_scheduled_at"] == nil || raw["sku_override"] != false {
					t.Fatalf("subscription projection = %#v", row)
				}
			} else if id != "2" || row["cancelled_at"] == nil {
				t.Fatalf("subscription projection = %#v", row)
			}
		case "subscription_metafields":
			if row["owner_id"] != "9007199254740993" || row["key"] != "gift" {
				t.Fatalf("metafield projection = %#v", row)
			}
		}
	}
	if counts["store"] != 1 || counts["subscriptions"] != 2 || counts["onetimes"] != 0 || counts["subscription_metafields"] != 1 {
		t.Fatalf("counts = %v", counts)
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Contains(requests, "/store?") {
		t.Fatalf("store request must carry no list parameters: %v", requests)
	}
}

func TestRechargeIncrementalUsesStringWatermarkAndSkipsBoundOnLaterPages(t *testing.T) {
	var mu sync.Mutex
	var queries []string
	src := rechargeTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		queries = append(queries, r.URL.RawQuery)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "" {
			fmt.Fprint(w, `{"next_cursor":"c2","previous_cursor":null,"orders":[{"id":1,"customer":{"id":10},"charge":{"id":100},"status":"success","type":"recurring","total_price":"20.00","created_at":"2026-01-01T00:00:00+00:00","updated_at":"2026-01-10T12:00:00+00:00"}]}`)
			return
		}
		fmt.Fprint(w, `{"next_cursor":null,"previous_cursor":"c1","orders":[{"id":2,"customer":{"id":11},"charge":{"id":101},"status":"queued","type":"checkout","total_price":"5.00","created_at":"2026-01-01T00:00:00+00:00","updated_at":"2026-01-09T12:00:00"}]}`)
	})
	prev := map[string]filament.Checkpoint{
		"orders": checkpoint.KeysetCheckpoint{
			Mode: checkpoint.ModeIncremental, Cols: []string{"orders_updated_at"}, Types: []string{"timestamptz"},
			Shards: []checkpoint.KeysetShard{{Key: []string{"2026-01-08T00:00:00+00:00"}}},
		}.ToCheckpoint("orders"),
	}
	plan, err := src.PlanIncremental(t.Context(), []string{"orders"}, prev, nil)
	if err != nil {
		t.Fatal(err)
	}
	var sink collectSink
	if err := src.ExtractFrom(t.Context(), &sink, filament.ExtractOpts{Resources: []string{"orders"}, Parallelism: 1}, plan); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(queries) != 2 || !strings.Contains(queries[0], "updated_at_min=2026-01-08T00%3A00%3A00%2B00%3A00") || queries[1] != "cursor=c2&limit=250" {
		t.Fatalf("queries = %v", queries)
	}
	if len(sink.records) != 2 {
		t.Fatalf("rows = %d", len(sink.records))
	}
	// Offset-suffixed and bare timestamps must both count, and the watermark
	// is the string maximum seen across pages.
	for _, rec := range sink.records {
		if !slices.Equal(rec.Key, []string{"2026-01-10T12:00:00+00:00"}) {
			t.Fatalf("watermark = %v", rec.Key)
		}
		var row map[string]any
		if err := json.Unmarshal(rec.Data, &row); err != nil {
			t.Fatal(err)
		}
		if row["customer_id"] == nil || row["charge_id"] == nil {
			t.Fatalf("order projection = %#v", row)
		}
	}
}
