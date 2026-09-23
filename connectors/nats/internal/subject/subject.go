// Package subject implements NATS subject pattern algebra shared by the
// JetStream source and sink.
package subject

import "strings"

// Intersect returns the most specific pattern matched by both a and b, or
// false when no subject satisfies both. Either side may use * and > wildcards.
func Intersect(a, b string) (string, bool) {
	x, y := strings.Split(a, "."), strings.Split(b, ".")
	out := []string{}
	for i := 0; ; i++ {
		if i == len(x) || i == len(y) {
			return strings.Join(out, "."), i == len(x) && i == len(y)
		}
		if x[i] == ">" {
			return strings.Join(append(out, y[i:]...), "."), true
		}
		if y[i] == ">" {
			return strings.Join(append(out, x[i:]...), "."), true
		}
		switch {
		case x[i] == y[i]:
			out = append(out, x[i])
		case x[i] == "*":
			out = append(out, y[i])
		case y[i] == "*":
			out = append(out, x[i])
		default:
			return "", false
		}
	}
}

// Covered reports whether any filter can match the pattern. A concrete subject
// is covered when a filter matches it exactly; a pattern such as prefix.> is
// covered when some filter overlaps it at all.
func Covered(filters []string, pattern string) bool {
	for _, filter := range filters {
		if _, ok := Intersect(filter, pattern); ok {
			return true
		}
	}
	return false
}
