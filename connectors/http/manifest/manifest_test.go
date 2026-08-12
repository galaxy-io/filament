package manifest

import (
	"testing"
)

func TestParseConciseSyntaxNormalizesToRuntimeModel(t *testing.T) {
	m, err := Parse([]byte(`
version: 1
name: concise
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

func TestParseConciseBasicAuth(t *testing.T) {
	m, err := Parse([]byte(`
version: 1
name: basic
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
