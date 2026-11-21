package manifest

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLastRecordPagination(t *testing.T) {
	const base = `version: 1
name: last_record_test
display_name: Last Record Test
description: Last record pagination test
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: https://example.com
resources:
  - name: items
    path: /items
    primary_key: [id]
    fields: {id: string}
    response:
      records: $
      pagination:
%s
`
	for _, tc := range []struct {
		name, pagination string
		valid            bool
	}{
		{
			"concise query",
			"        last_record: {field: id, request: query.before, page_size: 1000}",
			true,
		},
		{
			"concise header",
			"        last_record: {field: id, request: header.X-Before, page_size: 100}",
			true,
		},
		{
			"missing field",
			"        last_record: {request: query.before, page_size: 1000}",
			false,
		},
		{
			"bad request target",
			"        last_record: {field: id, request: cookie.before, page_size: 1000}",
			false,
		},
		{
			"bare request name",
			"        last_record: {field: id, request: before, page_size: 1000}",
			false,
		},
		{
			"page_size zero",
			"        last_record: {field: id, request: query.before, page_size: 0}",
			false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Parse([]byte(strings.Replace(base, "%s", tc.pagination, 1)))
			if (err == nil) != tc.valid {
				t.Fatalf("parse err=%v, want valid=%v", err, tc.valid)
			}
			if err != nil {
				return
			}
			p := m.Resources[0].Pagination
			if p.Type != "last_record" || p.LastRecordField != "id" || p.PageSize == 0 {
				t.Fatalf("pagination=%+v", p)
			}
		})
	}
}

func TestLastRecordPaginationExplicitForm(t *testing.T) {
	// The explicit form bypasses the concise grammar, so decode the
	// PaginationSpec directly rather than through Parse.
	var p PaginationSpec
	if err := yaml.Unmarshal([]byte(`type: last_record
last_record_field: id
inject_into: query
cursor_param: before
page_size: 1000
`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Type != "last_record" || p.LastRecordField != "id" || p.InjectInto != "query" || p.CursorParam != "before" || p.PageSize != 1000 {
		t.Fatalf("pagination=%+v", p)
	}
}
