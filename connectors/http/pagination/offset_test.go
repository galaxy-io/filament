package pagination

import (
	"net/http"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func TestOffsetPaginatorInjectsBodyFields(t *testing.T) {
	paginator, err := newOffset(manifest.PaginationSpec{
		Type:             "offset",
		OffsetParam:      "offset",
		LimitParam:       "limit",
		PageSize:         500,
		OffsetInjectInto: "body",
	})
	if err != nil {
		t.Fatalf("new offset paginator: %v", err)
	}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com/query", nil)
	overrides, err := paginator.Apply(req, State{Offset: 500})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if overrides["offset"] != 500 || overrides["limit"] != 500 {
		t.Fatalf("overrides = %#v, want offset and limit in body", overrides)
	}
	if req.URL.RawQuery != "" {
		t.Fatalf("query = %q, want empty", req.URL.RawQuery)
	}
}

func TestOffsetContinuationAcrossShortPages(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		spec := manifest.PaginationSpec{OffsetParam: "offset", LimitParam: "limit", PageSize: 50}
		if explicit {
			spec.HasMorePath = "meta.has_more"
		}
		p, err := newOffset(spec)
		if err != nil {
			t.Fatal(err)
		}
		state := p.Initial()
		for _, count := range []int{50, 0, 3} {
			req, _ := http.NewRequest(http.MethodGet, "https://example.com?status=completed", nil)
			_, err = p.Apply(req, state)
			if err != nil || req.URL.Query().Get("status") != "completed" || req.URL.Query().Get("limit") != "50" {
				t.Fatalf("request=%v err=%v", req, err)
			}
			next, err := p.Next(nil, map[string]any{"meta": map[string]any{"has_more": true}}, count)
			if err != nil {
				t.Fatal(err)
			}
			if explicit || count == 50 {
				if next.Done || next.Offset != state.Offset+50 {
					t.Fatalf("next=%+v, previous=%+v", next, state)
				}
			} else if !next.Done {
				t.Fatalf("default pagination must stop on short pages: %+v", next)
			}
			state = next
		}
	}
}
