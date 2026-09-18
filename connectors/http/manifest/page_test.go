package manifest

import (
	"fmt"
	"strings"
	"testing"
)

func TestPageTargets(t *testing.T) {
	const base = `version: 1
name: pages
display_name: Pages
description: Page tests
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: https://example.com
resources:
  - name: rows
    path: /rows
    primary_key: [id]
    fields:
      id: string
      updated_at: timestamptz
    response:
      records: $.rows
      pagination:
        page: {number: %s, size: %s, page_size: 50, more: pagination.more}
%s
`
	for _, tc := range []struct {
		number, size, target string
		valid                bool
	}{
		{"page", "limit", "query", true},
		{"query.page", "query.limit", "query", true},
		{"body.variables.page", "body.variables.limit", "body", true},
		{"body.page", "query.limit", "", false},
		{"header.page", "header.limit", "", false},
	} {
		t.Run(tc.number+tc.size, func(t *testing.T) {
			m, err := Parse([]byte(fmt.Sprintf(base, tc.number, tc.size, "")))
			if (err == nil) != tc.valid {
				t.Fatalf("parse: %v", err)
			}
			if err == nil {
				p := m.Resources[0].Pagination
				if p.InjectInto != tc.target || p.HasMorePath != "pagination.more" {
					t.Fatalf("pagination=%+v", p)
				}
			}
		})
	}
	for _, param := range []string{"variables.page", "variables.limit"} {
		inc := "    incremental: {cursor_field: updated_at, start_param: " + param + ", inject_into: body, comparator: time}"
		_, err := Parse([]byte(fmt.Sprintf(base, "body.variables.page", "body.variables.limit", inc)))
		if err == nil || !strings.Contains(err.Error(), "pagination") {
			t.Fatalf("expected body pagination collision, got %v", err)
		}
	}
}
