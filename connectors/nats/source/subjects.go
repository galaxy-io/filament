package source

import (
	"fmt"
	"github.com/galaxy-io/filament/connectors/nats/internal/subject"
	"slices"
	"strings"
	"unicode"
)

func validateSubject(subject string) error {
	tokens := strings.Split(subject, ".")
	for i, t := range tokens {
		if t == "" || strings.IndexFunc(t, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
			return fmt.Errorf("nats: invalid subject pattern %q", subject)
		}
		if t == ">" {
			if i != len(tokens)-1 {
				return fmt.Errorf("nats: > must be the final subject token")
			}
			continue
		}
		if t != "*" && strings.ContainsAny(t, "*>") {
			return fmt.Errorf("nats: wildcards must occupy a whole subject token")
		}
	}
	return nil
}

// subjectFilters returns the minimal filters covering pattern within a stream's
// stored subjects. Terminal > consumes one or more tokens; * exactly one.
func subjectFilters(pattern string, subjects []string) []string {
	var filters []string
	for _, stored := range subjects {
		if f, ok := subject.Intersect(pattern, stored); ok {
			filters = append(filters, f)
		}
	}
	slices.Sort(filters)
	filters = slices.Compact(filters)
	var out []string
	for i, f := range filters {
		redundant := false
		for j, g := range filters {
			if i != j {
				if intersection, ok := subject.Intersect(f, g); ok && intersection == f {
					redundant = true
					break
				}
			}
		}
		if !redundant {
			out = append(out, f)
		}
	}
	return out
}
