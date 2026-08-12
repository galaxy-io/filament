package server

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

const (
	// defaultPageSize applies when a pagination request omits a size.
	defaultPageSize = 25
	// maxPageSize caps the size a pagination request may ask for.
	maxPageSize = 1000
)

// encodeCursor encodes an offset as an opaque cursor. Offset <= 0 returns the
// empty cursor, the first page.
func encodeCursor(offset int32) string {
	if offset <= 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(int(offset))))
}

// decodeCursor decodes a cursor back to its offset. The empty cursor decodes
// to 0, the first page.
func decodeCursor(cursor string) (int32, error) {
	if cursor == "" {
		return 0, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor: %w", err)
	}
	offset, err := strconv.ParseInt(string(decoded), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor value: %w", err)
	}
	if offset < 0 {
		return 0, errors.New("cursor offset cannot be negative")
	}
	return int32(offset), nil
}

// pageSizeOf clamps the requested page size to (0, maxPageSize], falling back
// to defaultPageSize when unset.
func pageSizeOf(size int32) int32 {
	if size <= 0 {
		return defaultPageSize
	}
	if size > maxPageSize {
		return maxPageSize
	}
	return size
}

// paginationResponse reports the pre-page total plus cursors for the adjacent
// pages. Absent cursors mean no page in that direction.
func paginationResponse(offset, size, total int32) *ingestionv1.PaginationResponse {
	resp := &ingestionv1.PaginationResponse{Total: total}
	if offset > 0 {
		prev := encodeCursor(max(offset-size, 0))
		resp.PreviousCursor = &prev
	}
	if offset+size < total {
		next := encodeCursor(offset + size)
		resp.NextCursor = &next
	}
	return resp
}

// pageOf applies the request's cursor and page size to items and returns the
// page alongside the response pagination. A nil request means no pagination:
// the full slice comes back. An invalid cursor is an InvalidArgument error.
func pageOf[T any](items []T, p *ingestionv1.PaginationRequest) ([]T, *ingestionv1.PaginationResponse, error) {
	total := int32(len(items)) //nolint:gosec // list sizes fit int32
	if p == nil {
		return items, &ingestionv1.PaginationResponse{Total: total}, nil
	}
	offset, err := decodeCursor(p.GetCursor())
	if err != nil {
		return nil, nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	size := pageSizeOf(p.GetTotal())
	resp := paginationResponse(offset, size, total)
	if offset >= total {
		return nil, resp, nil
	}
	items = items[offset:]
	if int(size) < len(items) {
		items = items[:size]
	}
	return items, resp, nil
}
