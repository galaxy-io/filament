package model

import (
	"fmt"
	"strconv"
)

// drainPageSize sizes the pages behind DrainPages; deployments cap it
// server-side.
const drainPageSize int32 = 1000

// DrainPages collects every item behind a cursor-paged fetch.
func DrainPages[T any](fetch func(PageRequest) (Page[T], error)) ([]T, error) {
	var items []T
	cursor := ""
	for {
		page, err := fetch(PageRequest{PageSize: drainPageSize, Cursor: cursor})
		if err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
		cursor = page.NextCursor
		if cursor == "" {
			return items, nil
		}
	}
}

// Paginate cuts one offset-cursor page out of a fully loaded listing, for
// targets without deployment-native cursors.
func Paginate[T any](items []T, request PageRequest) (Page[T], error) {
	pageSize := int(request.PageSize)
	if pageSize <= 0 {
		pageSize = 25
	}
	offset := 0
	if request.Cursor != "" {
		parsed, err := strconv.Atoi(request.Cursor)
		if err != nil || parsed < 0 || parsed > len(items) {
			return Page[T]{}, fmt.Errorf("invalid page cursor")
		}
		offset = parsed
	}
	end := min(offset+pageSize, len(items))
	page := Page[T]{Items: items[offset:end], Total: len(items)}
	if end < len(items) {
		page.NextCursor = strconv.Itoa(end)
	}
	if offset > 0 {
		page.PreviousCursor = strconv.Itoa(max(offset-pageSize, 0))
	}
	return page, nil
}
