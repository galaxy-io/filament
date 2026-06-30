package request

import "testing"

func TestMergeOverridesSetsNestedBodyPath(t *testing.T) {
	got := MergeOverrides(map[string]any{
		"query":     "query",
		"variables": map[string]any{"first": 100},
	}, map[string]any{"variables.after": "cursor-1"})

	body := got.(map[string]any)
	vars := body["variables"].(map[string]any)
	if vars["first"] != 100 || vars["after"] != "cursor-1" {
		t.Fatalf("variables = %#v, want first and nested after", vars)
	}
}
