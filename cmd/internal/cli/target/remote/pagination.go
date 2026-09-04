package remote

import (
	"time"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// pageSize is the deployment's maximum, so a full collection costs as few
// round trips as the server allows.
const pageSize int32 = 1000

// pagination maps a page request onto the wire; a zero size means the
// deployment's default.
func pagination(request model.PageRequest) *ingestionv1.PaginationRequest {
	out := &ingestionv1.PaginationRequest{PageSize: request.PageSize}
	if request.Cursor != "" {
		out.Cursor = &request.Cursor
	}
	return out
}

func pageInfo(info *ingestionv1.PaginationResponse) model.PageInfo {
	return model.PageInfo{
		Total: int(info.GetTotal()), NextCursor: info.GetNextCursor(), PreviousCursor: info.GetPreviousCursor(),
	}
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
