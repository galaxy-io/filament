package httpapi

import (
	"net/http/httptest"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/template"
)

func TestSelectableAuthDefaultsAndRequiredFields(t *testing.T) {
	const data = `
version: 1
name: auth_fixture
display_name: Auth fixture
description: Selectable authentication fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
config:
  auth_method: {type: enum, enum: [api_key, access_token], default: api_key, enum_labels: {api_key: API Key}}
  api_key:
    type: secret
    required: true
    visible_when: {field: auth_method, values: [api_key]}
  access_token:
    type: secret
    required: true
    visible_when: {field: auth_method, values: [access_token]}
connection:
  base_url: https://example.com
  auth:
    select:
      field: auth_method
      cases:
        api_key:
          required: [api_key]
          auth: {header: {name: X-API-Key, value: config.api_key}}
        access_token:
          required: [access_token]
          auth: {bearer: config.access_token}
resources:
  - name: items
    path: /items
    primary_key: [id]
    fields: {id: string}
`
	for _, tc := range []struct {
		name         string
		values       map[string]any
		header, want string
		invalid      bool
	}{
		{"legacy default", map[string]any{"api_key": "key"}, "X-API-Key", "key", false},
		{"explicit key", map[string]any{"auth_method": "api_key", "api_key": "key"}, "X-API-Key", "key", false},
		{"token only", map[string]any{"auth_method": "access_token", "access_token": "token"}, "Authorization", "Bearer token", false},
		{"missing default credential", map[string]any{}, "", "", true},
		{"wrong credential", map[string]any{"auth_method": "access_token", "api_key": "key"}, "", "", true},
		{"blank credential", map[string]any{"api_key": "  "}, "", "", true},
		{"unknown method", map[string]any{"auth_method": "invalid", "api_key": "key"}, "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := NewManifest([]byte(data))
			cfg := filament.NewConfig(tc.values)
			err := src.Validate(cfg)
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid configuration accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if src.config.Fields[0].Name != "auth_method" || src.config.Fields[0].Enum[0].Label != "API Key" {
				t.Fatal("selector ordering or labels lost")
			}
			if err := src.Configure(t.Context(), cfg); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { src.Teardown(t.Context()) })
			req := httptest.NewRequest("GET", "https://example.com/items", nil)
			if err := src.connector.builder.Auth.Apply(t.Context(), req, template.Scope{Config: src.connector.creds}); err != nil {
				t.Fatal(err)
			}
			if got := req.Header.Get(tc.header); got != tc.want {
				t.Fatalf("header = %q, want %q", got, tc.want)
			}
		})
	}
}
