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
