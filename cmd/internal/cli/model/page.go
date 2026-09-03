package model

// drainPageSize sizes the pages behind DrainPages; targets cap it.
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
