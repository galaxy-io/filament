package remote

import (
	"time"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// pageSize is the deployment's maximum, so a full collection costs as few
// round trips as the server allows.
const pageSize int32 = 1000

func pagination(cursor string) *ingestionv1.PaginationRequest {
	request := &ingestionv1.PaginationRequest{PageSize: pageSize}
	if cursor != "" {
		request.Cursor = &cursor
	}
	return request
}

// drain collects every item behind a cursor-paged fetch.
func drain[T any](fetch func(cursor string) ([]T, *ingestionv1.PaginationResponse, error)) ([]T, error) {
	var items []T
	cursor := ""
	for {
		page, next, err := fetch(cursor)
		if err != nil {
			return nil, err
		}
		items = append(items, page...)
		cursor = next.GetNextCursor()
		if cursor == "" {
			return items, nil
		}
	}
}

func timeFromMillis(millis int64) time.Time {
	if millis == 0 {
		return time.Time{}
	}
	return time.UnixMilli(millis)
}
