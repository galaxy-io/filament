package pagination

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/internal/paths"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// cursorPaginator handles opaque-cursor pagination. The cursor returned in the
// response body is injected into the next request via body, query, or header.
//
// Termination tri-state for cursor_path:
//
//   - missing key  → terminate (most common: API drops the field on last page)
//   - empty string → terminate (some APIs send "" instead of dropping)
//   - explicit null → error, unless allow_null_terminates is set
//   - non-empty value → continue with that value as next state.Cursor
//
// has_more_path, when configured, takes precedence: missing/null/false all
// terminate. Combine the two when an API uses both signals defensively.
type cursorPaginator struct {
	cursorPath          string
	cursorParam         string
	injectInto          string // body | query | header
	hasMorePath         string
	allowNullTerminates bool
}

func newCursor(spec manifest.PaginationSpec) (*cursorPaginator, error) {
	if spec.CursorPath == "" {
		return nil, fmt.Errorf("cursor pagination: cursor_path is required")
	}
	if spec.CursorParam == "" {
		return nil, fmt.Errorf("cursor pagination: cursor_param is required")
	}
	inject := spec.InjectInto
	if inject == "" {
		inject = "body"
	}
	if inject != "body" && inject != "query" && inject != "header" {
		return nil, fmt.Errorf("cursor pagination: inject_into must be body|query|header (got %q)", inject)
	}
	return &cursorPaginator{
		cursorPath:          spec.CursorPath,
		cursorParam:         spec.CursorParam,
		injectInto:          inject,
		hasMorePath:         spec.HasMorePath,
		allowNullTerminates: spec.AllowNullTerminates,
	}, nil
}

func (p *cursorPaginator) Initial() State { return State{} }

func (p *cursorPaginator) Apply(req *http.Request, s State) (map[string]any, error) {
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

func (p *cursorPaginator) Next(_ *http.Response, body map[string]any, _ int) (State, error) {
	if p.hasMorePath != "" {
		more, err := paths.Bool(body, p.hasMorePath)
		switch {
		case errors.Is(err, errs.ErrPathMissing), errors.Is(err, errs.ErrPathNull):
			// Absent has_more is treated as false — pagination terminates.
			return State{Done: true}, nil
		case err != nil:
			return State{}, fmt.Errorf("cursor pagination: has_more_path %q: %w", p.hasMorePath, err)
		}
		if !more {
			return State{Done: true}, nil
		}
	}
	// Tri-state cursor extraction:
	//   - missing field → terminate (most APIs drop the field on the last page)
	//   - explicit null → terminate iff allow_null_terminates, else error
	//   - present scalar → coerce to string and continue (or terminate if empty)
	// Coercion handles numeric/bool cursors (e.g. last-id pagination).
	cur, present, err := paths.AsStringStrict(body, p.cursorPath)
	switch {
	case errors.Is(err, errs.ErrPathMissing):
		return State{Done: true}, nil
	case errors.Is(err, errs.ErrPathNull):
		if p.allowNullTerminates {
			return State{Done: true}, nil
		}
		return State{}, fmt.Errorf("cursor pagination: cursor_path %q resolved to null "+
			"(set allow_null_terminates: true to opt in): %w", p.cursorPath, err)
	case err != nil:
		return State{}, fmt.Errorf("cursor pagination: cursor_path %q: %w", p.cursorPath, err)
	case !present || cur == "":
		return State{Done: true}, nil
	}
	return State{Cursor: cur}, nil
}
