package pagination

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// offsetPaginator increments a numeric offset by page_size. Stops when the
// API returns fewer records than page_size (short page), unless has_more_path
// supplies an explicit continuation signal.
//
// Single-threaded per resource extraction: lastOffset tracks the offset most
// recently injected by Apply so that Next can advance it.
type offsetPaginator struct {
	offsetParam string
	limitParam  string
	pageSize    int
	injectInto  string
	lastOffset  int
	hasMorePath string
}

func newOffset(spec manifest.PaginationSpec) (*offsetPaginator, error) {
	if spec.OffsetParam == "" {
		return nil, fmt.Errorf("offset pagination: offset_param is required")
	}
	if spec.PageSize <= 0 {
		return nil, fmt.Errorf("offset pagination: page_size > 0 is required")
	}
	return &offsetPaginator{
		offsetParam: spec.OffsetParam,
		hasMorePath: spec.HasMorePath,
		limitParam:  spec.LimitParam,
		pageSize:    spec.PageSize,
		injectInto:  spec.OffsetInjectInto,
	}, nil
}

func (p *offsetPaginator) Initial() State { return State{Offset: 0} }

func (p *offsetPaginator) Apply(req *http.Request, s State) (map[string]any, error) {
	p.lastOffset = s.Offset
	if p.injectInto == "body" {
		out := map[string]any{p.offsetParam: s.Offset}
		if p.limitParam != "" {
			out[p.limitParam] = p.pageSize
		}
		return out, nil
	}
	q := req.URL.Query()
	q.Set(p.offsetParam, strconv.Itoa(s.Offset))
	if p.limitParam != "" {
		q.Set(p.limitParam, strconv.Itoa(p.pageSize))
	}
	req.URL.RawQuery = q.Encode()
	return nil, nil
}

func (p *offsetPaginator) Next(_ *http.Response, body map[string]any, recordCount int) (State, error) {
	if p.hasMorePath != "" {
		more, err := hasMore(body, p.hasMorePath)
		if err != nil {
			return State{}, fmt.Errorf("offset pagination: has_more_path %q: %w", p.hasMorePath, err)
		}
		if !more {
			return State{Done: true}, nil
		}
		return State{Offset: p.lastOffset + p.pageSize}, nil
	}
	if recordCount < p.pageSize {
		return State{Done: true}, nil
	}
	return State{Offset: p.lastOffset + p.pageSize}, nil
}
