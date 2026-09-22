package pagination

import (
	"net/http"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func TestPageInjectionAndTermination(t *testing.T) {
	for _, target := range []string{"", "query", "body"} {
		t.Run(target, func(t *testing.T) {
			p, err := newPage(manifest.PaginationSpec{PageParam: "variables.page", SizeParam: "variables.limit", PageSize: 50, InjectInto: target})
			if err != nil {
				t.Fatal(err)
			}
			if p.Initial().Page != 1 {
				t.Fatal("page must start at one")
			}
			req, _ := http.NewRequest(http.MethodPost, "https://example.com?keep=yes", nil)
			body, err := p.Apply(req, State{Page: 2})
			if err != nil {
				t.Fatal(err)
			}
			if target == "body" {
				if body["variables.page"] != 2 || body["variables.limit"] != 50 || req.URL.RawQuery != "keep=yes" {
					t.Fatalf("body=%v query=%s", body, req.URL.RawQuery)
				}
			} else if body != nil || req.URL.Query().Get("variables.page") != "2" || req.URL.Query().Get("variables.limit") != "50" || req.URL.Query().Get("keep") != "yes" {
				t.Fatalf("body=%v query=%s", body, req.URL.RawQuery)
			}
			next, err := p.Next(nil, nil, 50)
			if err != nil || next.Page != 3 {
				t.Fatalf("next=%+v err=%v", next, err)
			}
			next, err = p.Next(nil, nil, 49)
			if err != nil || !next.Done {
				t.Fatalf("next=%+v err=%v", next, err)
			}
		})
	}
}

func TestPageExplicitMoreOverridesShortPages(t *testing.T) {
	p, err := newPage(manifest.PaginationSpec{PageParam: "page", PageSize: 50, HasMorePath: "pagination.more"})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	_, _ = p.Apply(req, State{Page: 3})
	for _, count := range []int{0, 1, 50} {
		next, err := p.Next(nil, map[string]any{"pagination": map[string]any{"more": true}}, count)
		if err != nil || next.Page != 4 || next.Done {
			t.Fatalf("count=%d next=%+v err=%v", count, next, err)
		}
	}
}

func TestPageAndCursorContinuation(t *testing.T) {
	for _, strategy := range []string{"page", "cursor"} {
		for _, tc := range []struct {
			name       string
			body       map[string]any
			done, fail bool
		}{
			{"true", map[string]any{"more": true, "cursor": "next"}, false, false},
			{"false", map[string]any{"more": false, "cursor": "next"}, true, false},
			{"missing", map[string]any{"cursor": "next"}, true, false},
			{"null", map[string]any{"more": nil, "cursor": "next"}, true, false},
			{"wrong_type", map[string]any{"more": "true", "cursor": "next"}, false, true},
		} {
			t.Run(strategy+"/"+tc.name, func(t *testing.T) {
				p, err := New(manifest.PaginationSpec{Type: strategy, PageParam: "page", PageSize: 50, CursorParam: "cursor", CursorPath: "cursor", HasMorePath: "more"})
				if err != nil {
					t.Fatal(err)
				}
				next, err := p.Next(nil, tc.body, 50)
				if (err != nil) != tc.fail || next.Done != tc.done {
					t.Fatalf("next=%+v err=%v; want done=%v fail=%v", next, err, tc.done, tc.fail)
				}
			})
		}
	}
}
