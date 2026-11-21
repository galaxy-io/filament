package pagination

import (
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func TestCursorStopOnEmptyIsOffByDefault(t *testing.T) {
	p, err := newCursor(manifest.PaginationSpec{CursorPath: "next", CursorParam: "cursor", InjectInto: "query"})
	if err != nil {
		t.Fatalf("newCursor: %v", err)
	}
	state, err := p.Next(nil, map[string]any{"next": "abc"}, nil)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if state.Done || state.Cursor != "abc" {
		t.Fatalf("state = %+v, want to continue with cursor abc on an empty page", state)
	}
}

func TestCursorStopOnEmptyStopsBeforeReadingCursorOrMore(t *testing.T) {
	p, err := newCursor(manifest.PaginationSpec{
		CursorPath: "next", CursorParam: "cursor", InjectInto: "query",
		HasMorePath: "has_more", StopOnEmpty: true,
	})
	if err != nil {
		t.Fatalf("newCursor: %v", err)
	}
	body := map[string]any{"next": "abc", "has_more": true}
	state, err := p.Next(nil, body, []map[string]any{})
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if !state.Done {
		t.Fatalf("state = %+v, want Done on a page with no records", state)
	}
	state, err = p.Next(nil, body, []map[string]any{{"id": "1"}})
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if state.Done || state.Cursor != "abc" {
		t.Fatalf("state = %+v, want to continue with cursor abc on a page with records", state)
	}
}
