package pagination

import (
	"net/http"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func intPtr(v int) *int { return &v }

func TestPageStartsAtConfiguredPage(t *testing.T) {
	p, err := newPage(manifest.PaginationSpec{PageParam: "page", PageSize: 2, FirstPage: intPtr(0)})
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Initial(); got.Page != 0 || got.Done {
		t.Fatalf("initial = %+v, want page 0", got)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if _, err := p.Apply(req, p.Initial()); err != nil {
		t.Fatal(err)
	}
	if got := req.URL.Query().Get("page"); got != "0" {
		t.Fatalf("page = %q, want 0", got)
	}
	next, err := p.Next(nil, nil, 2)
	if err != nil || next.Page != 1 {
		t.Fatalf("next = %+v err = %v", next, err)
	}
}

// total_pages counts pages, so a walk that starts at 0 stops after index
// total-1 instead of requesting one page past the end.
func TestPageTotalPagesWithZeroStart(t *testing.T) {
	p, err := newPage(manifest.PaginationSpec{PageParam: "page", PageSize: 2, FirstPage: intPtr(0), TotalPagesPath: "pages"})
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"pages": 2}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	_, _ = p.Apply(req, State{Page: 0})
	if next, err := p.Next(nil, body, 2); err != nil || next.Done || next.Page != 1 {
		t.Fatalf("after page 0: next = %+v err = %v", next, err)
	}
	_, _ = p.Apply(req, State{Page: 1})
	if next, err := p.Next(nil, body, 2); err != nil || !next.Done {
		t.Fatalf("after page 1: next = %+v err = %v, want done", next, err)
	}
}

// Mixpanel Engage returns a session_id on the first page and rejects any
// later page without it.
func TestPageSessionCarriedToLaterPages(t *testing.T) {
	for _, target := range []string{"query", "body", "header"} {
		t.Run(target, func(t *testing.T) {
			p, err := newPage(manifest.PaginationSpec{
				PageParam: "page", PageSize: 2, FirstPage: intPtr(0), InjectInto: "body",
				SessionPath: "session_id", SessionParam: "session_id", SessionInjectInto: target,
			})
			if err != nil {
				t.Fatal(err)
			}
			req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
			first, err := p.Apply(req, p.Initial())
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := first["session_id"]; ok || req.URL.Query().Has("session_id") || req.Header.Get("session_id") != "" {
				t.Fatal("first page must not send a session")
			}
			next, err := p.Next(nil, map[string]any{"session_id": "abc"}, 2)
			if err != nil || next.Page != 1 || next.Cursor != "abc" {
				t.Fatalf("next = %+v err = %v", next, err)
			}
			req, _ = http.NewRequest(http.MethodPost, "https://example.com", nil)
			body, err := p.Apply(req, next)
			if err != nil {
				t.Fatal(err)
			}
			if body["page"] != 1 {
				t.Fatalf("body = %v, want page 1", body)
			}
			var got string
			switch target {
			case "query":
				got = req.URL.Query().Get("session_id")
			case "body":
				got, _ = body["session_id"].(string)
			case "header":
				got = req.Header.Get("session_id")
			}
			if got != "abc" {
				t.Fatalf("%s session = %q, want abc", target, got)
			}
		})
	}
}

func TestPageSessionMissingFailsTheWalk(t *testing.T) {
	p, err := newPage(manifest.PaginationSpec{PageParam: "page", PageSize: 2, SessionPath: "session_id", SessionParam: "session_id", SessionInjectInto: "query"})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	_, _ = p.Apply(req, p.Initial())
	if _, err := p.Next(nil, map[string]any{}, 2); err == nil {
		t.Fatal("a full page without a session token continued")
	}
	if next, err := p.Next(nil, map[string]any{}, 1); err != nil || !next.Done {
		t.Fatalf("short last page: next = %+v err = %v, want done without a token", next, err)
	}
}

func TestPageRejectsInvalidStartAndHalfSession(t *testing.T) {
	if _, err := newPage(manifest.PaginationSpec{PageParam: "page", PageSize: 2, FirstPage: intPtr(-1)}); err == nil {
		t.Fatal("negative start accepted")
	}
	if _, err := newPage(manifest.PaginationSpec{PageParam: "page", PageSize: 2, SessionPath: "session_id"}); err == nil {
		t.Fatal("session without a request parameter accepted")
	}
}
