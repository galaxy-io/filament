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

// pagePaginator increments a 1-based page number. Stops on a short page or
// when total_pages is reached (if total_pages_path is configured).
type pagePaginator struct {
	pageParam      string
	sizeParam      string
	pageSize       int
	totalPagesPath string
	lastPage       int
}

func newPage(spec manifest.PaginationSpec) (*pagePaginator, error) {
	if spec.PageParam == "" {
		return nil, fmt.Errorf("page pagination: page_param is required")
	}
	if spec.PageSize <= 0 {
		return nil, fmt.Errorf("page pagination: page_size > 0 is required")
	}
	return &pagePaginator{
		pageParam:      spec.PageParam,
		sizeParam:      spec.SizeParam,
		pageSize:       spec.PageSize,
		totalPagesPath: spec.TotalPagesPath,
	}, nil
}

func (p *pagePaginator) Initial() State { return State{Page: 1} }

func (p *pagePaginator) Apply(req *http.Request, s State) (map[string]any, error) {
	p.lastPage = s.Page
	q := req.URL.Query()
	q.Set(p.pageParam, strconv.Itoa(s.Page))
	if p.sizeParam != "" {
		q.Set(p.sizeParam, strconv.Itoa(p.pageSize))
	}
	req.URL.RawQuery = q.Encode()
	return nil, nil
}

func (p *pagePaginator) Next(_ *http.Response, body map[string]any, recordCount int) (State, error) {
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
		if int64(p.lastPage) >= total {
			return State{Done: true}, nil
		}
	}
	return State{Page: p.lastPage + 1}, nil
}
