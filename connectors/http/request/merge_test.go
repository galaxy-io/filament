package request

import "testing"

func TestMergeOverridesSetsNestedBodyPath(t *testing.T) {
	got, err := MergeOverrides(map[string]any{
		"query":     "query",
		"variables": map[string]any{"first": 100},
	}, map[string]any{"variables.after": "cursor-1"})

	if err != nil {
		t.Fatal(err)
	}
	body := got.(map[string]any)
	vars := body["variables"].(map[string]any)
	if vars["first"] != 100 || vars["after"] != "cursor-1" {
		t.Fatalf("variables = %#v, want first and nested after", vars)
	}
}

func TestMergeOverridesArrayFilters(t *testing.T) {
	body := map[string]any{"filters": []any{map[string]any{"operator": "GTE", "value": "old"}, map[string]any{"operator": "GT", "value": "0"}}}
	got, err := MergeOverrides(body, map[string]any{"filters.0.value": "date", "filters.1.value": "42"})
	if err != nil {
		t.Fatal(err)
	}
	filters := got.(map[string]any)["filters"].([]any)
	if filters[0].(map[string]any)["value"] != "date" || filters[1].(map[string]any)["value"] != "42" {
		t.Fatalf("filters=%v", filters)
	}
	if _, err = MergeOverrides(body, map[string]any{"filters.2.value": "missing"}); err == nil {
		t.Fatal("invalid filter index silently ignored")
	}
}
