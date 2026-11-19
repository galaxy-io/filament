package template

import (
	"errors"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/errs"
)

func TestParseAndRender(t *testing.T) {
	scope := Scope{
		Config: map[string]string{"api_key": "secret123", "subdomain": "acme"},
		Parent: map[string]string{"id": "parent-456"},
		Cursor: "cursor_token_xyz",
	}

	tests := []struct {
		name    string
		tmpl    string
		want    string
		wantErr bool
	}{
		{
			name: "plain text without placeholders",
			tmpl: "https://api.example.com/v1/users",
			want: "https://api.example.com/v1/users",
		},
		{
			name: "single placeholder",
			tmpl: "Bearer {{ config.api_key }}",
			want: "Bearer secret123",
		},
		{
			name: "multiple placeholders",
			tmpl: "https://{{ config.subdomain }}.example.com/parents/{{ parent.id }}/items",
			want: "https://acme.example.com/parents/parent-456/items",
		},
		{
			name: "cursor placeholder",
			tmpl: "/items?page={{ cursor }}",
			want: "/items?page=cursor_token_xyz",
		},
		{
			name:    "missing key without default errors",
			tmpl:    "{{ config.missing_key }}",
			wantErr: true,
		},
		{
			name:    "cursor with sub-key errors",
			tmpl:    "{{ cursor.invalid }}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.tmpl, scope)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Render(%q) error = %v, wantErr %v", tt.tmpl, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Render(%q) = %q, want %q", tt.tmpl, got, tt.want)
			}
		})
	}
}

func TestDefaultFilter(t *testing.T) {
	scope := Scope{
		Config: map[string]string{"existing": "real-value"},
	}

	t.Run("applies default when key is missing", func(t *testing.T) {
		got, err := Render(`{{ config.missing | default "fallback-val" }}`, scope)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "fallback-val" {
			t.Errorf("got %q, want %q", got, "fallback-val")
		}
	})

	t.Run("uses existing value when key is present", func(t *testing.T) {
		got, err := Render(`{{ config.existing | default "fallback-val" }}`, scope)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "real-value" {
			t.Errorf("got %q, want %q", got, "real-value")
		}
	})

	t.Run("handles escaped quotes inside default", func(t *testing.T) {
		got, err := Render(`{{ config.missing | default "hello \"world\"" }}`, scope)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != `hello "world"` {
			t.Errorf("got %q, want %q", got, `hello "world"`)
		}
	})
}

func TestBasenameFilter(t *testing.T) {
	scope := Scope{
		Parent: map[string]string{
			"event_uri":            "https://api.calendly.com/scheduled_events/4e62-89cd",
			"trailing_slash":       "https://api.calendly.com/scheduled_events/4e62-89cd/",
			"with_query":           "https://api.calendly.com/scheduled_events/4e62-89cd?status=active",
			"with_fragment":        "https://api.calendly.com/scheduled_events/4e62-89cd#invitees",
			"relative_path":        "/scheduled_events/4e62-89cd",
			"relative_simple":      "scheduled_events/4e62-89cd",
			"single_id":            "4e62-89cd",
			"encoded_slash_url":    "https://api.example.com/events/team%2Fmember",
			"encoded_slash_rel":    "/events/team%2Fmember",
			"path_with_spaces_url": "https://api.example.com/events/team member",
			"path_with_spaces_rel": "/events/team member",
			"plain_spaces":         "team member",
			"url_empty_path_query": "https://api.example.com?status=active",
			"url_empty_path":       "https://api.example.com",
			"url_slash_only":       "https://api.example.com/",
			"root_only":            "/",
			"empty_val":            "",
			"dot":                  ".",
			"dotdot":               "..",
			"dot_path":             "/a/.",
			"dotdot_path":          "/a/..",
		},
	}

	tests := []struct {
		name    string
		tmpl    string
		want    string
		wantErr bool
	}{
		{
			name: "canonical full URI",
			tmpl: "/scheduled_events/{{ parent.event_uri | basename }}/invitees",
			want: "/scheduled_events/4e62-89cd/invitees",
		},
		{
			name: "full URI with trailing slash",
			tmpl: "{{ parent.trailing_slash | basename }}",
			want: "4e62-89cd",
		},
		{
			name: "full URI with query params",
			tmpl: "{{ parent.with_query | basename }}",
			want: "4e62-89cd",
		},
		{
			name: "full URI with fragment",
			tmpl: "{{ parent.with_fragment | basename }}",
			want: "4e62-89cd",
		},
		{
			name: "relative path",
			tmpl: "{{ parent.relative_path | basename }}",
			want: "4e62-89cd",
		},
		{
			name: "relative path without leading slash",
			tmpl: "{{ parent.relative_simple | basename }}",
			want: "4e62-89cd",
		},
		{
			name: "single ID segment passes through",
			tmpl: "{{ parent.single_id | basename }}",
			want: "4e62-89cd",
		},
		{
			name: "preserves encoded slash %2F in URL",
			tmpl: "{{ parent.encoded_slash_url | basename }}",
			want: "team%2Fmember",
		},
		{
			name: "preserves encoded slash %2F in relative path",
			tmpl: "{{ parent.encoded_slash_rel | basename }}",
			want: "team%2Fmember",
		},
		{
			name: "path-escapes spaces in URL segment",
			tmpl: "{{ parent.path_with_spaces_url | basename }}",
			want: "team%20member",
		},
		{
			name: "path-escapes spaces in relative segment",
			tmpl: "{{ parent.path_with_spaces_rel | basename }}",
			want: "team%20member",
		},
		{
			name: "path-escapes spaces in plain ID",
			tmpl: "{{ parent.plain_spaces | basename }}",
			want: "team%20member",
		},
		{
			name:    "url with query and empty path errors",
			tmpl:    "{{ parent.url_empty_path_query | basename }}",
			wantErr: true,
		},
		{
			name:    "url with empty path errors",
			tmpl:    "{{ parent.url_empty_path | basename }}",
			wantErr: true,
		},
		{
			name:    "url with root slash only errors",
			tmpl:    "{{ parent.url_slash_only | basename }}",
			wantErr: true,
		},
		{
			name:    "root path / errors",
			tmpl:    "{{ parent.root_only | basename }}",
			wantErr: true,
		},
		{
			name:    "empty string errors",
			tmpl:    "{{ parent.empty_val | basename }}",
			wantErr: true,
		},
		{
			name:    "rejects dot segment",
			tmpl:    "{{ parent.dot | basename }}",
			wantErr: true,
		},
		{
			name:    "rejects dotdot segment",
			tmpl:    "{{ parent.dotdot | basename }}",
			wantErr: true,
		},
		{
			name:    "rejects path ending in dot",
			tmpl:    "{{ parent.dot_path | basename }}",
			wantErr: true,
		},
		{
			name:    "rejects path ending in dotdot",
			tmpl:    "{{ parent.dotdot_path | basename }}",
			wantErr: true,
		},
		{
			name:    "missing parent key errors",
			tmpl:    "{{ parent.missing | basename }}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.tmpl, scope)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Render(%q) error = %v, wantErr %v", tt.tmpl, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Render(%q) = %q, want %q", tt.tmpl, got, tt.want)
			}
		})
	}
}

func TestChainedFilters(t *testing.T) {
	scope := Scope{
		Parent: map[string]string{
			"event_uri": "https://api.calendly.com/scheduled_events/4e62-89cd",
		},
	}

	t.Run("present key with basename and default", func(t *testing.T) {
		got, err := Render(`{{ parent.event_uri | basename | default "fallback" }}`, scope)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "4e62-89cd" {
			t.Errorf("got %q, want %q", got, "4e62-89cd")
		}
	})

	t.Run("missing key with basename then default", func(t *testing.T) {
		got, err := Render(`{{ parent.missing | basename | default "fallback" }}`, scope)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "fallback" {
			t.Errorf("got %q, want %q", got, "fallback")
		}
	})

	t.Run("missing key with default URL then basename", func(t *testing.T) {
		got, err := Render(`{{ parent.missing | default "https://api.example.com/events/fallback-id" | basename }}`, scope)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "fallback-id" {
			t.Errorf("got %q, want %q", got, "fallback-id")
		}
	})
}

func TestFilterSyntaxErrors(t *testing.T) {
	tests := []struct {
		name string
		tmpl string
	}{
		{
			name: "unknown filter",
			tmpl: `{{ config.key | unknown_filter }}`,
		},
		{
			name: "unclosed pipe",
			tmpl: `{{ config.key | }}`,
		},
		{
			name: "default without string",
			tmpl: `{{ config.key | default }}`,
		},
		{
			name: "default with unquoted argument",
			tmpl: `{{ config.key | default raw_val }}`,
		},
		{
			name: "basename with arguments rejected",
			tmpl: `{{ config.key | basename "arg" }}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.tmpl)
			if err == nil {
				t.Fatalf("Parse(%q) expected error, got nil", tt.tmpl)
			}
			if !errors.Is(err, errs.ErrTemplateSyntax) {
				t.Errorf("Parse(%q) error = %v, want ErrTemplateSyntax", tt.tmpl, err)
			}
		})
	}
}

func TestValidateScopes(t *testing.T) {
	tmpl := "{{ config.api_key }} and {{ parent.id }}"
	if err := Validate(tmpl, []string{"config", "parent"}); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}

	if err := Validate(tmpl, []string{"config"}); err == nil {
		t.Error("Validate() expected error for parent scope in config-only allowed list, got nil")
	}
}
