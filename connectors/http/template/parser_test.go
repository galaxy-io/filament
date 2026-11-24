package template

import (
	"testing"
)

func TestParseAndRender(t *testing.T) {
	tests := []struct {
		name     string
		template string
		scope    Scope
		want     string
		wantErr  bool
	}{
		{
			name:     "plain text",
			template: "/sites/items",
			scope:    Scope{},
			want:     "/sites/items",
		},
		{
			name:     "simple placeholder",
			template: "/sites/{{ parent.id }}/sitemaps",
			scope: Scope{
				Parent: map[string]string{"id": "12345"},
			},
			want: "/sites/12345/sitemaps",
		},
		{
			name:     "default filter fallback",
			template: "{{ config.host | default \"https://default.com\" }}",
			scope:    Scope{},
			want:     "https://default.com",
		},
		{
			name:     "default filter with present value",
			template: "{{ config.host | default \"https://default.com\" }}",
			scope: Scope{
				Config: map[string]string{"host": "https://custom.com"},
			},
			want: "https://custom.com",
		},
		{
			name:     "url_encode filter with URL-prefix",
			template: "/webmasters/v3/sites/{{ parent.site_url | url_encode }}/sitemaps",
			scope: Scope{
				Parent: map[string]string{"site_url": "https://www.example.com/"},
			},
			want: "/webmasters/v3/sites/https%3A%2F%2Fwww.example.com%2F/sitemaps",
		},
		{
			name:     "url_encode filter with domain property",
			template: "/webmasters/v3/sites/{{ parent.site_url | url_encode }}/searchAnalytics/query",
			scope: Scope{
				Parent: map[string]string{"site_url": "sc-domain:example.com"},
			},
			want: "/webmasters/v3/sites/sc-domain%3Aexample.com/searchAnalytics/query",
		},
		{
			name:     "url_encode filter with spaces",
			template: "/search?q={{ config.query | url_encode }}",
			scope: Scope{
				Config: map[string]string{"query": "hello world/test"},
			},
			want: "/search?q=hello%20world%2Ftest",
		},
		{
			name:     "default and url_encode combined",
			template: "{{ config.url | default \"https://fallback.com/path\" | url_encode }}",
			scope:    Scope{},
			want:     "https%3A%2F%2Ffallback.com%2Fpath",
		},
		{
			name:     "missing key without default",
			template: "{{ parent.missing }}",
			scope:    Scope{},
			wantErr:  true,
		},
		{
			name:     "unknown filter",
			template: "{{ config.key | unknown }}",
			scope:    Scope{},
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Render(tc.template, tc.scope)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Render(%q) err = %v, wantErr = %v", tc.template, err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("Render(%q) = %q, want %q", tc.template, got, tc.want)
			}
		})
	}
}
