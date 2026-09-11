package manifest

import (
	"strings"
	"testing"
)

func TestPendingResponsePollingDefaultsCanBeOverridden(t *testing.T) {
	m, err := Parse([]byte(`
version: 1
name: async
display_name: Async
description: Asynchronous response test.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: https://example.com
defaults:
  response:
    poll_pending: true
resources:
  - name: computed
    path: /computed
  - name: immediate
    path: /immediate
    response:
      poll_pending: false
`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Resources[0].Response.PollPending == nil || !*m.Resources[0].Response.PollPending {
		t.Fatal("pending-response polling default was not inherited")
	}
	if m.Resources[1].Response.PollPending == nil || *m.Resources[1].Response.PollPending {
		t.Fatal("explicit false did not disable inherited polling")
	}
}

func TestParseCatalogMetadata(t *testing.T) {
	m, err := Parse([]byte(`
version: 1
name: example
display_name: Example API
description: Example connector used to verify manifest-owned catalog metadata.
dark_logo_url: https://cdn.example.com/example-dark.svg
light_logo_url: https://cdn.example.com/example-light.svg
connection:
  base_url: https://example.com
resources:
  - name: records
    path: /records
`))
	if err != nil {
		t.Fatalf("parse manifest metadata: %v", err)
	}
	if m.DisplayName != "Example API" ||
		m.Description != "Example connector used to verify manifest-owned catalog metadata." ||
		m.DarkLogoURL != "https://cdn.example.com/example-dark.svg" ||
		m.LightLogoURL != "https://cdn.example.com/example-light.svg" {
		t.Fatalf("catalog metadata = %#v", m)
	}
}

func TestParseConciseSyntaxNormalizesToRuntimeModel(t *testing.T) {
	m, err := Parse([]byte(`
version: 1
name: concise
display_name: Concise
description: Test concise manifest.
dark_logo_url: https://cdn.example.com/concise-dark.svg
light_logo_url: https://cdn.example.com/concise-light.svg
config:
  token:
    type: secret
    required: true
    help: API token
defaults:
  method: GET
  headers:
    Accept: application/json
  query:
    limit: "100"
  response:
    records: $
    pagination:
      link: next
field_sets:
  timestamps:
    created_at: timestamptz
    updated_at: timestamptz?
connection:
  base_url: https://example.com
  auth:
    bearer: config.token
resources:
  - name: parents
    path: /parents
    records: $
    primary_key: [id]
    fields:
      id: int64
      updated_at: timestamptz?
      payload: { path: data, type: json, nullable: true }
    capture:
      id: id
  - name: children
    path: /parents/{parent_id}/children
    params:
      parent_id: parent.id
    for_each: parents
    records: $.data.items
    query:
      limit: "25"
    primary_key: [id]
    use_fields: [timestamps]
    exclude_fields: [created_at]
    fields:
      id: string
discovery:
  mode: static
`))
	if err != nil {
		t.Fatalf("parse concise manifest: %v", err)
	}
	if field := m.Config["token"]; field.Type != "secret" || !field.Required || field.Help != "API token" {
		t.Fatalf("config token = %#v", field)
	}
	if m.Connection.Auth.Type != "bearer" || m.Connection.Auth.Params["token"] != "{{ config.token }}" {
		t.Fatalf("concise auth = %#v", m.Connection.Auth)
	}
	parent := m.Resources[0]
	if parent.Method != "GET" || parent.Headers["Accept"] != "application/json" ||
		parent.Query["limit"] != "100" || parent.Response.Root != "array" ||
		parent.Pagination.Type != "link_header" {
		t.Fatalf("parent defaults not inherited: %#v", parent)
	}
	if parent.Fields[0].Name != "id" || parent.Fields[0].Path != "id" || parent.Fields[0].Type != "int64" {
		t.Fatalf("id shorthand = %#v", parent.Fields[0])
	}
	if !parent.Fields[1].Nullable || parent.Fields[1].Type != "timestamptz" {
		t.Fatalf("nullable shorthand = %#v", parent.Fields[1])
	}
	child := m.Resources[1]
	if child.Parent == nil || child.Parent.Resource != "parents" {
		t.Fatalf("for_each normalization = %#v", child.Parent)
	}
	if child.Path != "/parents/{{ parent.id }}/children" {
		t.Fatalf("typed path params = %q", child.Path)
	}
	if child.Response.Root != "object" || child.Response.RecordsPath != "data.items" {
		t.Fatalf("records path normalization = %#v", child.Response)
	}
	if child.Query["limit"] != "25" {
		t.Fatalf("resource query did not override default: %#v", child.Query)
	}
	if len(child.Fields) != 2 || child.Fields[0].Name != "updated_at" || child.Fields[1].Name != "id" {
		t.Fatalf("field set expansion = %#v", child.Fields)
	}
	if m.Discovery.Mode != "static" || len(m.Discovery.Resources) != 0 {
		t.Fatalf("static discovery = %#v", m.Discovery)
	}
}

func TestParseListConfig(t *testing.T) {
	m, err := Parse([]byte(`
version: 1
name: list-config
display_name: List Config
description: Test list configuration manifest.
dark_logo_url: https://cdn.example.com/list-dark.svg
light_logo_url: https://cdn.example.com/list-light.svg
config:
  kinds:
    type: list
    default: [public, private]
    enum: [public, private, direct]
connection:
  base_url: https://example.com
resources:
  - name: records
    path: /records
    records: $
    primary_key: [id]
    fields:
      id: string
`))
	if err != nil {
		t.Fatalf("parse list config: %v", err)
	}
	field := m.Config["kinds"]
	if field.Type != "list" || len(field.Enum) != 3 {
		t.Fatalf("list config = %#v", field)
	}
}

func TestParseRejectsListConfigWithoutOptions(t *testing.T) {
	_, err := Parse([]byte(`
version: 1
name: list-config
display_name: List Config
description: Test invalid list configuration manifest.
dark_logo_url: https://cdn.example.com/list-dark.svg
light_logo_url: https://cdn.example.com/list-light.svg
config:
  kinds:
    type: list
connection:
  base_url: https://example.com
resources:
  - name: records
    path: /records
    records: $
    primary_key: [id]
    fields:
      id: string
`))
	if err == nil || !strings.Contains(err.Error(), "enum") {
		t.Fatalf("parse error = %v, want missing enum", err)
	}
}

func TestParseRejectsInvalidIncrementalContracts(t *testing.T) {
	tests := []struct {
		name, field, pagination, incremental, want string
	}{
		{name: "unprojected cursor", field: "id: string", incremental: "cursor_field: updated_at\n      start_param: since\n      inject_into: query\n      comparator: time", want: "must be declared and projected"},
		{name: "nullable cursor", field: "updated_at: timestamptz?", incremental: "cursor_field: updated_at\n      start_param: since\n      inject_into: query\n      comparator: time", want: "must be non-nullable"},
		{name: "incompatible comparator", field: "updated_at: json", incremental: "cursor_field: updated_at\n      start_param: since\n      inject_into: query\n      comparator: time", want: "is incompatible"},
		{name: "pagination collision", field: "updated_at: timestamptz", pagination: "pagination:\n      cursor:\n        response: next\n        request: query.since", incremental: "cursor_field: updated_at\n      start_param: since\n      inject_into: query\n      comparator: time", want: "conflicts with the pagination injection"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte("version: 1\nname: test\ndisplay_name: Test\ndescription: Test invalid incremental manifest.\ndark_logo_url: https://cdn.example.com/test-dark.svg\nlight_logo_url: https://cdn.example.com/test-light.svg\nconnection:\n  base_url: https://example.com\nresources:\n  - name: items\n    path: /items\n    fields:\n      " + tt.field + "\n    " + tt.pagination + "\n    incremental:\n      " + tt.incremental + "\n"))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseRejectsInvalidCaptureOnlyAndSince(t *testing.T) {
	const head = "version: 1\nname: test\ndisplay_name: Test\ndescription: Test capture-only manifest.\ndark_logo_url: https://cdn.example.com/test-dark.svg\nlight_logo_url: https://cdn.example.com/test-light.svg\nconnection:\n  base_url: https://example.com\nresources:\n"
	const incremental = "    incremental:\n      cursor_field: ts\n      start_param: oldest\n      inject_into: query\n      comparator: numeric\n"
	tests := []struct{ name, body, want string }{
		{
			name: "capture_only without capture",
			body: "  - name: threads\n    path: /threads\n    capture_only: true\n",
			want: "requires a capture block",
		},
		{
			name: "capture_only with incremental",
			body: "  - name: threads\n    path: /threads\n    capture_only: true\n    capture:\n      ts: ts\n    fields:\n      ts: string\n" + incremental,
			want: "cannot be combined with incremental",
		},
		{
			name: "capture_only in discovery include",
			body: "  - name: threads\n    path: /threads\n    capture_only: true\n    capture:\n      ts: ts\ndiscovery:\n  mode: static\n  include: [threads]\n",
			want: "is capture_only",
		},
		{
			name: "since without incremental",
			body: "  - name: threads\n    path: /threads\n    capture:\n      latest_reply: latest_reply\n  - name: replies\n    path: /replies\n    for_each: threads\n    parent:\n      since: latest_reply\n",
			want: "requires an incremental block",
		},
		{
			name: "since not captured by parent",
			body: "  - name: threads\n    path: /threads\n    capture:\n      ts: ts\n  - name: replies\n    path: /replies\n    for_each: threads\n    parent:\n      since: latest_reply\n    fields:\n      ts: string\n" + incremental,
			want: "is not captured by parent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(head + tt.body))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseForEachPairsWithParentSince(t *testing.T) {
	m, err := Parse([]byte("version: 1\nname: test\ndisplay_name: Test\ndescription: Test since manifest.\ndark_logo_url: https://cdn.example.com/test-dark.svg\nlight_logo_url: https://cdn.example.com/test-light.svg\nconnection:\n  base_url: https://example.com\nresources:\n  - name: threads\n    path: /threads\n    capture_only: true\n    capture:\n      latest_reply: latest_reply\n  - name: replies\n    path: /replies\n    for_each: threads\n    parent:\n      since: latest_reply\n    fields:\n      ts: string\n    incremental:\n      cursor_field: ts\n      start_param: oldest\n      inject_into: query\n      comparator: numeric\n"))
	if err != nil {
		t.Fatal(err)
	}
	replies := m.Resources[1]
	if replies.Parent == nil || replies.Parent.Resource != "threads" || replies.Parent.Since != "latest_reply" {
		t.Fatalf("parent = %#v, want for_each resource with since", replies.Parent)
	}
}

func TestParseConciseBasicAuth(t *testing.T) {
	m, err := Parse([]byte(`
version: 1
name: basic
display_name: Basic
description: Test basic authentication manifest.
dark_logo_url: https://cdn.example.com/basic-dark.svg
light_logo_url: https://cdn.example.com/basic-light.svg
config:
  email:
    type: string
    required: true
  api_token:
    type: secret
    required: true
connection:
  base_url: https://example.com
  auth:
    basic:
      username: config.email
      password: config.api_token
resources:
  - name: widgets
    path: /widgets
`))
	if err != nil {
		t.Fatalf("parse basic auth manifest: %v", err)
	}
	if m.Connection.Auth.Type != "basic" ||
		m.Connection.Auth.Params["user"] != "{{ config.email }}" ||
		m.Connection.Auth.Params["pass"] != "{{ config.api_token }}" {
		t.Fatalf("concise basic auth = %#v", m.Connection.Auth)
	}

	m, err = Parse([]byte(`
version: 1
name: basic-no-pass
display_name: Basic No Password
description: Test basic authentication without a password.
dark_logo_url: https://cdn.example.com/basic-dark.svg
light_logo_url: https://cdn.example.com/basic-light.svg
config:
  api_key:
    type: secret
    required: true
connection:
  base_url: https://example.com
  auth:
    basic:
      username: config.api_key
resources:
  - name: widgets
    path: /widgets
`))
	if err != nil {
		t.Fatalf("parse basic auth without password: %v", err)
	}
	if m.Connection.Auth.Params["user"] != "{{ config.api_key }}" || m.Connection.Auth.Params["pass"] != "" {
		t.Fatalf("basic auth without password = %#v", m.Connection.Auth)
	}
}

func TestParseConciseFieldRejectsUnknownType(t *testing.T) {
	_, err := Parse([]byte(`
version: 1
name: invalid
display_name: Invalid
description: Test invalid authentication manifest.
dark_logo_url: https://cdn.example.com/invalid-dark.svg
light_logo_url: https://cdn.example.com/invalid-light.svg
connection:
  base_url: https://example.com
resources:
  - name: widgets
    path: /widgets
    fields:
      id: imaginary?
`))
	if err == nil {
		t.Fatalf("error = %v, want shorthand type validation", err)
	}
}

// base_url is rendered once at Configure, so it carries auth's scope set:
// config/state/env resolve, parent/cursor never can. Regional APIs use this to
// pick a host from config instead of pinning one cloud per manifest.
func TestParseAcceptsTemplatedBaseURL(t *testing.T) {
	m, err := Parse([]byte(`
version: 1
name: regional
display_name: Regional
description: Test regional host manifest.
dark_logo_url: https://cdn.example.com/regional-dark.svg
light_logo_url: https://cdn.example.com/regional-light.svg
config:
  host:
    type: string
    default: https://us.example.com
connection:
  base_url: "{{ config.host }}"
resources:
  - name: widgets
    path: /widgets
`))
	if err != nil {
		t.Fatalf("parse templated base_url: %v", err)
	}
	if m.Connection.BaseURL != "{{ config.host }}" {
		t.Fatalf("base_url = %q, want the template preserved for render at Configure", m.Connection.BaseURL)
	}
}

func TestParseRejectsBaseURLWithUnresolvableScope(t *testing.T) {
	_, err := Parse([]byte(`
version: 1
name: regional
display_name: Regional
description: Test invalid regional host manifest.
dark_logo_url: https://cdn.example.com/regional-dark.svg
light_logo_url: https://cdn.example.com/regional-light.svg
connection:
  base_url: "{{ parent.host }}"
resources:
  - name: widgets
    path: /widgets
`))
	if err == nil {
		t.Fatal("base_url referencing parent parsed; that scope can never resolve at connection time")
	}
}

func TestParseRejectsFormerV3Manifest(t *testing.T) {
	_, err := Parse([]byte(`
version: 3
name: legacy
connection:
  base_url: https://example.com
resources:
  - name: widgets
    path: /widgets
`))
	if err == nil {
		t.Fatal("former v3 manifest parsed; v1 is the only supported contract")
	}
}

func TestRateLimitResponseValidation(t *testing.T) {
	for _, tc := range []struct {
		name, rule string
		valid      bool
	}{
		{"complete", `{status: 403, header: Budget, header_value: '0', body_path: error.detail, body_contains: exhausted, backoff_seconds: 90}`, true},
		{"missing status", `{header: Budget}`, false},
		{"header value without header", `{status: 403, header_value: '0'}`, false},
		{"body path without condition", `{status: 403, body_path: error.detail}`, false},
		{"body condition without path", `{status: 403, body_contains: exhausted}`, false},
		{"empty body condition", `{status: 403, body_path: error.detail, body_contains: ''}`, false},
		{"negative backoff", `{status: 403, backoff_seconds: -1}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(`
version: 1
name: test
display_name: Test
description: Rate limit configuration test.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: https://example.com
  rate_limit:
    responses:
      - ` + tc.rule + `
resources:
  - name: items
    path: /items
`))
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}
