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
