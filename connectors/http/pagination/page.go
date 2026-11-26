package pagination

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/internal/paths"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// pagePaginator increments a page number that starts at 1, or at the
// manifest's start. Stops on a short page or when total_pages is reached (if
// total_pages_path is configured). An explicit has_more_path takes precedence
// over page length and total_pages. A configured session carries a token from
// each response into the next request, held in State.Cursor so a checkpoint
// resumes with it.
type pagePaginator struct {
	pageParam      string
	sizeParam      string
	pageSize       int
	start          int
	totalPagesPath string
	lastPage       int
	injectInto     string
	hasMorePath    string

	sessionPath   string
	sessionParam  string
	sessionInject string
}

func newPage(spec manifest.PaginationSpec) (*pagePaginator, error) {
	if spec.PageParam == "" {
		return nil, fmt.Errorf("page pagination: page_param is required")
	}
	if spec.PageSize <= 0 {
		return nil, fmt.Errorf("page pagination: page_size > 0 is required")
	}
	start := 1
	if spec.FirstPage != nil {
		start = *spec.FirstPage
	}
	if start < 0 {
		return nil, fmt.Errorf("page pagination: start must be 0 or greater")
	}
	if (spec.SessionPath == "") != (spec.SessionParam == "") {
		return nil, fmt.Errorf("page pagination: session requires both a response path and a request parameter")
	}
	return &pagePaginator{
		pageParam:      spec.PageParam,
		injectInto:     spec.InjectInto,
		hasMorePath:    spec.HasMorePath,
		sizeParam:      spec.SizeParam,
		pageSize:       spec.PageSize,
		start:          start,
		totalPagesPath: spec.TotalPagesPath,
		sessionPath:    spec.SessionPath,
		sessionParam:   spec.SessionParam,
		sessionInject:  spec.SessionInjectInto,
	}, nil
}

func (p *pagePaginator) Initial() State { return State{Page: p.start} }

func (p *pagePaginator) Apply(req *http.Request, s State) (map[string]any, error) {
	p.lastPage = s.Page
	var out map[string]any
	if p.injectInto == "body" {
		out = map[string]any{p.pageParam: s.Page}
		if p.sizeParam != "" {
			out[p.sizeParam] = p.pageSize
		}
	} else {
		q := req.URL.Query()
		q.Set(p.pageParam, strconv.Itoa(s.Page))
		if p.sizeParam != "" {
			q.Set(p.sizeParam, strconv.Itoa(p.pageSize))
		}
		req.URL.RawQuery = q.Encode()
	}
	if p.sessionParam == "" || s.Cursor == "" {
		return out, nil
	}
	switch p.sessionInject {
	case "body":
		if out == nil {
			out = map[string]any{}
		}
		out[p.sessionParam] = s.Cursor
	case "header":
		req.Header.Set(p.sessionParam, s.Cursor)
	default:
		q := req.URL.Query()
		q.Set(p.sessionParam, s.Cursor)
		req.URL.RawQuery = q.Encode()
	}
	return out, nil
}

// next advances to the following page, carrying the session token read from
// this page's body. A page that continues the walk without a token fails,
// because the API would reject or restart the next request.
func (p *pagePaginator) next(body map[string]any) (State, error) {
	state := State{Page: p.lastPage + 1}
	if p.sessionPath == "" {
		return state, nil
	}
	token, _, err := paths.AsString(body, p.sessionPath)
	if err != nil {
		return State{}, fmt.Errorf("page pagination: session %q: %w", p.sessionPath, err)
	}
	if token == "" {
		return State{}, fmt.Errorf("page pagination: session %q missing from a page with more results", p.sessionPath)
	}
	state.Cursor = token
	return state, nil
}

func (p *pagePaginator) Next(_ *http.Response, body map[string]any, recordCount int) (State, error) {
	if p.hasMorePath != "" {
		more, err := hasMore(body, p.hasMorePath)
		if err != nil {
			return State{}, fmt.Errorf("page pagination: has_more_path %q: %w", p.hasMorePath, err)
		}
		if !more {
			return State{Done: true}, nil
		}
		return p.next(body)
	}
	if recordCount == 0 || recordCount < p.pageSize {
		return State{Done: true}, nil
	}
	if p.totalPagesPath != "" {
		total, err := paths.Int(body, p.totalPagesPath)
		switch {
		case errors.Is(err, errs.ErrPathMissing), errors.Is(err, errs.ErrPathNull):
			// Manifest declared total_pages_path but the server didn't
			// include it; without this value the loop has no termination
			// condition.
			return State{}, fmt.Errorf(
				"page pagination: total_pages_path %q not present in response (would loop forever)",
				p.totalPagesPath)
		case err != nil:
			return State{}, fmt.Errorf("page pagination: total_pages_path %q: %w", p.totalPagesPath, err)
		}
		if int64(p.lastPage-p.start+1) >= total {
			return State{Done: true}, nil
		}
	}
	return p.next(body)
}
