package server

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

const (
	// defaultPageSize applies when a pagination request omits a size.
	defaultPageSize = 25
	// maxPageSize caps the size a pagination request may ask for.
	maxPageSize     = 1000
	maxSearchLength = 256
)

// encodeCursor encodes an offset as an opaque cursor. Offset <= 0 returns the
// empty cursor, the first page.
func encodeCursor(offset int32) string {
	if offset <= 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(int(offset))))
}

func listOptionsOf(
	p *ingestionv1.PaginationRequest,
	search string,
	sorting *ingestionv1.SortingRequest,
	fields map[ingestionv1.SortBy]string,
	defaultField string,
	defaultDescending bool,
) (filament.ListOptions, error) {
	search = strings.TrimSpace(search)
	if len(search) > maxSearchLength {
		return filament.ListOptions{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("search exceeds %d characters", maxSearchLength))
	}
	field := defaultField
	descending := defaultDescending
	if sorting != nil {
		if sorting.GetSortBy() != ingestionv1.SortBy_SORT_BY_UNSPECIFIED {
			mapped, ok := fields[sorting.GetSortBy()]
			if !ok {
				return filament.ListOptions{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unsupported sort field %s", sorting.GetSortBy()))
			}
			field = mapped
		}
		switch sorting.GetSortOrder() {
		case ingestionv1.SortOrder_SORT_ORDER_UNSPECIFIED,
			ingestionv1.SortOrder_SORT_ORDER_DESC:
			descending = true
		case ingestionv1.SortOrder_SORT_ORDER_ASC:
			descending = false
		default:
			return filament.ListOptions{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unsupported sort order %d", sorting.GetSortOrder()))
		}
	}

	options := filament.ListOptions{Search: search, SortBy: field, SortDescending: descending}
	if p != nil {
		offset, err := decodeCursor(p.GetCursor())
		if err != nil {
			return filament.ListOptions{}, connect.NewError(connect.CodeInvalidArgument, err)
		}
		options.Limit = int(pageSizeOf(p.GetPageSize()))
		options.Offset = int(offset)
	}
	return options, nil
}

func paginationOf(p *ingestionv1.PaginationRequest, options filament.ListOptions, total int) *ingestionv1.PaginationResponse {
	if p == nil {
		return &ingestionv1.PaginationResponse{Total: int32(total)} //nolint:gosec // datastore row counts fit int32
	}
	return paginationResponse(int32(options.Offset), int32(options.Limit), int32(total)) //nolint:gosec // API pagination is int32
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
