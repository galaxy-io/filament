package memory

import "strings"

func matchesSearch(search string, fields ...string) bool {
	search = strings.ToLower(strings.TrimSpace(search))
	if search == "" {
		return true
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), search) {
			return true
		}
	}
	return false
}

func pageSlice[T any](items []T, offset, limit int) []T {
	if offset >= len(items) {
		return nil
	}
	if offset > 0 {
		items = items[offset:]
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func ordered(comparison int, descending bool) int {
	if descending {
		return -comparison
	}
	return comparison
}
