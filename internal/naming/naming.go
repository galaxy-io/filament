// Package naming normalizes display names into destination identifiers.
package naming

import "strings"

// Normalize converts a display name into a conservative SQL-style identifier:
// lowercase ASCII letters, digits, and underscores. Runs of any other
// characters collapse to a single underscore, edges are trimmed, a leading
// digit gains an underscore prefix, and the result is capped at 63 bytes (the
// strictest destination limit, postgres). Returns "" when nothing usable
// remains.
func Normalize(name string) string {
	var b strings.Builder
	pending := false
	for _, r := range strings.ToLower(name) {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			pending = true
			continue
		}
		if pending && b.Len() > 0 {
			b.WriteByte('_')
		}
		pending = false
		b.WriteRune(r)
	}
	s := b.String()
	if s == "" {
		return ""
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "_" + s
	}
	if len(s) > 63 {
		s = strings.TrimRight(s[:63], "_")
	}
	return s
}
