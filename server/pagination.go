package server

import (
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// pageOf applies the request's limit/offset to items and returns the page
// alongside the response pagination carrying the pre-slice total. A nil
// request means no pagination: the full slice comes back.
func pageOf[T any](items []T, p *ingestionv1.PaginationRequest) ([]T, *ingestionv1.PaginationResponse) {
	resp := &ingestionv1.PaginationResponse{Total: int32(len(items))} //nolint:gosec // list sizes fit int32
	if p == nil {
		return items, resp
	}
	if offset := int(p.GetOffset()); offset > 0 {
		if offset >= len(items) {
			return nil, resp
		}
		items = items[offset:]
	}
	if limit := int(p.GetLimit()); limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, resp
}
