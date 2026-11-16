package template_test

import (
	"errors"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/template"
)

func TestRenderBasenameFilter(t *testing.T) {
	scope := template.Scope{
		Parent: map[string]string{
			"event_uri":    "https://api.calendly.com/scheduled_events/12345678-abcd-1234-abcd-123456789abc",
			"trailing_uri": "https://api.calendly.com/scheduled_events/12345678-abcd/",
			"plain_id":     "evt_998877",
			"empty_val":    "",
		},
	}

	cases := []struct {
		name string
		tmpl string
		want string
	}{
		{
			name: "canonical URI extracts trailing UUID",
			tmpl: "/scheduled_events/{{ parent.event_uri | basename }}/invitees",
			want: "/scheduled_events/12345678-abcd-1234-abcd-123456789abc/invitees",
		},
		{
			name: "trailing slash is stripped before basename",
			tmpl: "/scheduled_events/{{ parent.trailing_uri | basename }}/invitees",
			want: "/scheduled_events/12345678-abcd/invitees",
		},
		{
			name: "plain ID without slashes returns unchanged",
			tmpl: "/events/{{ parent.plain_id | basename }}",
			want: "/events/evt_998877",
		},
		{
			name: "empty value renders empty string",
			tmpl: "/events/{{ parent.empty_val | basename }}",
			want: "/events/",
		},
		{
			name: "combined default and basename filters",
			tmpl: "/events/{{ parent.missing_uri | default \"https://api.calendly.com/scheduled_events/fallback-id\" | basename }}",
			want: "/events/fallback-id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := template.Render(tc.tmpl, scope)
			if err != nil {
				t.Fatalf("Render(%q) unexpected error: %v", tc.tmpl, err)
			}
			if got != tc.want {
				t.Fatalf("Render(%q) = %q, want %q", tc.tmpl, got, tc.want)
			}
		})
	}
}

func TestParseBasenameRejectsArguments(t *testing.T) {
	_, err := template.Parse("{{ parent.event_uri | basename \"arg\" }}")
	if !errors.Is(err, errs.ErrTemplateSyntax) {
		t.Fatalf("expected ErrTemplateSyntax when basename is given an argument, got %v", err)
	}
}
