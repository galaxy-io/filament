package pagination

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// records builds n records with sequential string ids, the last one carrying
// the given id.
func lastRecordFixtures(t *testing.T, n int, lastID any) []map[string]any {
	t.Helper()
	records := make([]map[string]any, n)
	for i := range records {
		records[i] = map[string]any{"id": fmt.Sprintf("id-%d", i)}
	}
	if n > 0 {
		records[n-1]["id"] = lastID
	}
	return records
}

func TestLastRecordQueryInjectionAndEmptyCursorNoOp(t *testing.T) {
	p, err := newLastRecord(manifest.PaginationSpec{
		Type: "last_record", LastRecordField: "id", CursorParam: "before",
		InjectInto: "query", PageSize: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/cards?limit=1000", nil)
	if _, err := p.Apply(req, State{}); err != nil {
		t.Fatal(err)
	}
	if got := req.URL.Query().Get("before"); got != "" {
		t.Fatalf("before = %q, want empty on first page", got)
	}
	if _, err := p.Apply(req, State{Cursor: "abc123"}); err != nil {
		t.Fatal(err)
	}
	if got := req.URL.Query().Get("before"); got != "abc123" {
		t.Fatalf("before = %q, want abc123", got)
	}
	if got := req.URL.Query().Get("limit"); got != "1000" {
		t.Fatalf("limit = %q, want the original query preserved", got)
	}
}

func TestLastRecordNextEndsOnEmptyPage(t *testing.T) {
	p, err := newLastRecord(manifest.PaginationSpec{
		Type: "last_record", LastRecordField: "id", CursorParam: "before",
		InjectInto: "query", PageSize: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	next, err := p.Next(nil, nil, nil)
	if err != nil || !next.Done {
		t.Fatalf("next=%+v err=%v, want Done on an empty page", next, err)
	}
}

func TestLastRecordNextEndsOnShortPage(t *testing.T) {
	p, err := newLastRecord(manifest.PaginationSpec{
		Type: "last_record", LastRecordField: "id", CursorParam: "before",
		InjectInto: "query", PageSize: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	next, err := p.Next(nil, nil, lastRecordFixtures(t, 2, "last-id"))
	if err != nil || !next.Done {
		t.Fatalf("next=%+v err=%v, want Done on a short page", next, err)
	}
}

func TestLastRecordNextContinuesWithLastRecordID(t *testing.T) {
	p, err := newLastRecord(manifest.PaginationSpec{
		Type: "last_record", LastRecordField: "id", CursorParam: "before",
		InjectInto: "query", PageSize: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/cards", nil)
	if _, err := p.Apply(req, State{}); err != nil {
		t.Fatal(err)
	}
	next, err := p.Next(nil, nil, lastRecordFixtures(t, 3, "last-id"))
	if err != nil || next.Done || next.Cursor != "last-id" {
		t.Fatalf("next=%+v err=%v, want Cursor last-id", next, err)
	}
}

func TestLastRecordNextErrorsOnMissingField(t *testing.T) {
	p, err := newLastRecord(manifest.PaginationSpec{
		Type: "last_record", LastRecordField: "id", CursorParam: "before",
		InjectInto: "query", PageSize: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	records := []map[string]any{
		{"id": "a"},
		{"name": "no id here"},
	}
	if _, err := p.Next(nil, nil, records); err == nil {
		t.Fatal("want an error when the last record of a full page lacks the field")
	}
}

func TestLastRecordNextErrorsOnNullAndEmptyField(t *testing.T) {
	for _, value := range []any{nil, ""} {
		p, err := newLastRecord(manifest.PaginationSpec{
			Type: "last_record", LastRecordField: "id", CursorParam: "before",
			InjectInto: "query", PageSize: 2,
		})
		if err != nil {
			t.Fatal(err)
		}
		records := []map[string]any{
			{"id": "a"},
			{"id": value},
		}
		if _, err := p.Next(nil, nil, records); err == nil {
			t.Fatalf("want an error when the last record field is %v", value)
		}
	}
}

func TestLastRecordNextErrorsOnRepeatedCursor(t *testing.T) {
	p, err := newLastRecord(manifest.PaginationSpec{
		Type: "last_record", LastRecordField: "id", CursorParam: "before",
		InjectInto: "query", PageSize: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/cards", nil)
	if _, err := p.Apply(req, State{Cursor: "same-id"}); err != nil {
		t.Fatal(err)
	}
	records := []map[string]any{
		{"id": "first"},
		{"id": "same-id"},
	}
	if _, err := p.Next(nil, nil, records); err == nil {
		t.Fatal("want an error when the new cursor repeats the previous one")
	}
}

func TestLastRecordCoercesNumericField(t *testing.T) {
	p, err := newLastRecord(manifest.PaginationSpec{
		Type: "last_record", LastRecordField: "seq", CursorParam: "before",
		InjectInto: "query", PageSize: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	records := []map[string]any{
		{"seq": 41},
		{"seq": 42},
	}
	next, err := p.Next(nil, nil, records)
	if err != nil || next.Cursor != "42" {
		t.Fatalf("next=%+v err=%v, want Cursor 42", next, err)
	}
}

func TestLastRecordHeaderAndBodyInjection(t *testing.T) {
	p, err := newLastRecord(manifest.PaginationSpec{
		Type: "last_record", LastRecordField: "id", CursorParam: "X-Before",
		InjectInto: "header", PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/cards", nil)
	if _, err := p.Apply(req, State{Cursor: "hdr-id"}); err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("X-Before"); got != "hdr-id" {
		t.Fatalf("X-Before = %q, want hdr-id", got)
	}

	p, err = newLastRecord(manifest.PaginationSpec{
		Type: "last_record", LastRecordField: "id", CursorParam: "before",
		InjectInto: "body", PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest(http.MethodPost, "https://example.com/cards", nil)
	overrides, err := p.Apply(req, State{Cursor: "body-id"})
	if err != nil {
		t.Fatal(err)
	}
	if overrides["before"] != "body-id" {
		t.Fatalf("overrides = %#v, want before body-id", overrides)
	}
	if _, err := p.Apply(req, State{}); err != nil {
		t.Fatal(err)
	}
}

func TestLastRecordRejectsInvalidSpec(t *testing.T) {
	for _, spec := range []manifest.PaginationSpec{
		{Type: "last_record", CursorParam: "before", InjectInto: "query", PageSize: 10},
		{Type: "last_record", LastRecordField: "id", InjectInto: "query", PageSize: 10},
		{Type: "last_record", LastRecordField: "id", CursorParam: "before", InjectInto: "query", PageSize: 0},
		{Type: "last_record", LastRecordField: "id", CursorParam: "before", InjectInto: "cookie", PageSize: 10},
	} {
		if _, err := newLastRecord(spec); err == nil {
			t.Fatalf("spec=%+v, want an error", spec)
		}
	}
}
