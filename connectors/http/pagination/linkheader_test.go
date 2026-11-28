package pagination

import (
	"net/http"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func TestParseLinkHeader(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		targetRel string
		want      string
	}{
		{
			name:      "standard next link",
			header:    `<https://api.example.com/items?page=2>; rel="next"`,
			targetRel: "next",
			want:      "https://api.example.com/items?page=2",
		},
		{
			name:      "multiple links with target rel",
			header:    `<https://api.example.com/items?page=1>; rel="prev", <https://api.example.com/items?page=3>; rel="next"`,
			targetRel: "next",
			want:      "https://api.example.com/items?page=3",
		},
		{
			name:      "sentry link with results true",
			header:    `<https://sentry.io/api/0/organizations/acme/issues/?cursor=100:0:1>; rel="next"; results="true"; cursor="100:0:1"`,
			targetRel: "next",
			want:      "https://sentry.io/api/0/organizations/acme/issues/?cursor=100:0:1",
		},
		{
			name:      "sentry link with results false terminates",
			header:    `<https://sentry.io/api/0/organizations/acme/issues/?cursor=100:0:1>; rel="next"; results="false"; cursor="100:0:1"`,
			targetRel: "next",
			want:      "",
		},
		{
			name:      "sentry mixed links where next has results false and prev has true",
			header:    `<https://sentry.io/api/0/organizations/acme/issues/?cursor=100:0:0>; rel="previous"; results="true", <https://sentry.io/api/0/organizations/acme/issues/?cursor=100:0:1>; rel="next"; results="false"`,
			targetRel: "next",
			want:      "",
		},
		{
			name:      "no matching rel",
			header:    `<https://api.example.com/items?page=1>; rel="first", <https://api.example.com/items?page=9>; rel="last"`,
			targetRel: "next",
			want:      "",
		},
		{
			name:      "unquoted rel and results",
			header:    `<https://api.example.com/items?page=2>; rel=next; results=true`,
			targetRel: "next",
			want:      "https://api.example.com/items?page=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLinkHeader(tt.header, tt.targetRel)
			if got != tt.want {
				t.Errorf("parseLinkHeader(%q, %q) = %q, want %q", tt.header, tt.targetRel, got, tt.want)
			}
		})
	}
}

func TestLinkHeaderPaginator_NextStopsOnResultsFalse(t *testing.T) {
	p := newLinkHeader(manifest.PaginationSpec{Rel: "next"})
	resp := &http.Response{
		Header: http.Header{
			"Link": []string{
				`<https://sentry.io/api/0/organizations/acme/issues/?cursor=100:0:1>; rel="next"; results="false"`,
			},
		},
	}

	state, err := p.Next(resp, nil, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !state.Done {
		t.Fatalf("expected state.Done to be true, got State: %+v", state)
	}
}
