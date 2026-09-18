package source

import (
	"fmt"
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

// intersectSubjects returns the common subject language. Terminal > consumes
// one or more tokens; * consumes exactly one.
func intersectSubjects(a, b string) (string, bool) {
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

func subjectFilters(pattern string, subjects []string) []string {
	var filters []string
	for _, subject := range subjects {
		if f, ok := intersectSubjects(pattern, subject); ok {
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
				if intersection, ok := intersectSubjects(f, g); ok && intersection == f {
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
