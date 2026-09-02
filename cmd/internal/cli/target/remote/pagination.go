package remote

import ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"

// internalPageSize matches the deployment's maximum and minimizes round
// trips when an operation requires a complete collection.
const internalPageSize int32 = 1000

func paginationRequest(pageSize int32, cursor string) *ingestionv1.PaginationRequest {
	request := &ingestionv1.PaginationRequest{PageSize: pageSize}
	if cursor != "" {
		request.Cursor = &cursor
	}
	return request
}

// drainPages collects every item behind a cursor-paged proto fetch.
func drainPages[T any](fetch func(cursor string) ([]T, *ingestionv1.PaginationResponse, error)) ([]T, error) {
	var items []T
	cursor := ""
	for {
		page, pagination, err := fetch(cursor)
		if err != nil {
			return nil, err
		}
		items = append(items, page...)
		cursor = pagination.GetNextCursor()
		if cursor == "" {
			return items, nil
		}
	}
}
