package pagination

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/internal/paths"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// lastRecordPaginator pages root-array endpoints that cap at a fixed number
// of rows and offer no next-page token. The cursor for the next request is
// taken from a field of the last record on the current page and injected via
// query, header, or body (Trello's `before` parameter is the canonical case).
//
// Termination: a page shorter than page_size ends the walk, so page_size
// must match the limit the manifest requests. A full page whose last record
// lacks the field, or repeats the previous cursor, is an error, never a
// silent stop: either condition means the walk cannot make progress.
//
// Single-threaded per resource extraction: lastCursor tracks the cursor most
// recently injected by Apply so that Next can detect a repeated cursor.
type lastRecordPaginator struct {
	field       string
	cursorParam string
	injectInto  string // body | query | header
	pageSize    int
	lastCursor  string
}

func newLastRecord(spec manifest.PaginationSpec) (*lastRecordPaginator, error) {
	if spec.LastRecordField == "" {
		return nil, fmt.Errorf("last_record pagination: field is required")
	}
	if spec.CursorParam == "" {
		return nil, fmt.Errorf("last_record pagination: request parameter is required")
	}
	inject := spec.InjectInto
	if inject == "" {
		inject = "query"
	}
	if inject != "body" && inject != "query" && inject != "header" {
		return nil, fmt.Errorf("last_record pagination: inject_into must be body|query|header (got %q)", inject)
	}
	if spec.PageSize <= 0 {
		return nil, fmt.Errorf("last_record pagination: page_size > 0 is required")
	}
	return &lastRecordPaginator{
		field:       spec.LastRecordField,
		cursorParam: spec.CursorParam,
		injectInto:  inject,
		pageSize:    spec.PageSize,
	}, nil
}

func (p *lastRecordPaginator) Initial() State { return State{} }

func (p *lastRecordPaginator) Apply(req *http.Request, s State) (map[string]any, error) {
	p.lastCursor = s.Cursor
	if s.Cursor == "" {
		return nil, nil
	}
	switch p.injectInto {
	case "query":
		q := req.URL.Query()
		q.Set(p.cursorParam, s.Cursor)
		req.URL.RawQuery = q.Encode()
	case "header":
		req.Header.Set(p.cursorParam, s.Cursor)
	case "body":
		return map[string]any{p.cursorParam: s.Cursor}, nil
	}
	return nil, nil
}

func (p *lastRecordPaginator) Next(_ *http.Response, _ map[string]any, records []map[string]any) (State, error) {
	if len(records) < p.pageSize {
		return State{Done: true}, nil
	}
	last := records[len(records)-1]
	cur, present, err := paths.AsStringStrict(last, p.field)
	switch {
	case errors.Is(err, errs.ErrPathMissing):
		return State{}, fmt.Errorf("last_record pagination: field %q missing on the last record of a full page", p.field)
	case errors.Is(err, errs.ErrPathNull):
		return State{}, fmt.Errorf("last_record pagination: field %q is null on the last record of a full page", p.field)
	case err != nil:
		return State{}, fmt.Errorf("last_record pagination: field %q: %w", p.field, err)
	case !present || cur == "":
		return State{}, fmt.Errorf("last_record pagination: field %q is empty on the last record of a full page", p.field)
	}
	if cur == p.lastCursor {
		return State{}, fmt.Errorf("last_record pagination: field %q repeated the previous cursor %q (would loop forever)", p.field, cur)
	}
	return State{Cursor: cur}, nil
}
