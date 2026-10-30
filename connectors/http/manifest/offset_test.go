package manifest

import (
	"fmt"
	"testing"
)

func TestOffsetMore(t *testing.T) {
	const base = `version: 1
name: offset_test
display_name: Offset Test
description: Offset pagination test
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
      records: $.data
      pagination:
        offset: {offset: offset, limit: limit, page_size: 50%s}
`
	for _, tc := range []struct {
		suffix, path string
		valid        bool
	}{
		{"", "", true},
		{", more: meta.has_more", "meta.has_more", true},
		{", more: ''", "", false},
		{", more: true", "", false},
	} {
		t.Run(tc.suffix, func(t *testing.T) {
			m, err := Parse([]byte(fmt.Sprintf(base, tc.suffix)))
			if (err == nil) != tc.valid {
				t.Fatalf("parse: %v", err)
			}
			if err == nil && m.Resources[0].Pagination.HasMorePath != tc.path {
				t.Fatalf("pagination=%+v", m.Resources[0].Pagination)
			}
		})
	}
}
