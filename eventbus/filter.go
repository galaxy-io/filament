package eventbus

import (
	"fmt"
	"strings"
)

// Filter is a subscription pattern compiled once at Subscribe so delivery
// matches without re-parsing. Semantics match [SubjectMatch].
type Filter struct {
	pattern string
	tokens  []string
	tail    bool // pattern ends in '>'
}

// Compile parses a pattern into a Filter, erroring if it is empty or has '>'
// anywhere but the final token.
func Compile(pattern string) (Filter, error) {
	if pattern == "" {
		return Filter{}, fmt.Errorf("%w: empty", ErrBadPattern)
	}
	toks := strings.Split(pattern, Separator)
	tail := false
	for i, t := range toks {
		if t == TailWildcard {
			if i != len(toks)-1 {
				return Filter{}, fmt.Errorf("%w: %q: '>' must be the final token", ErrBadPattern, pattern)
			}
			tail = true
		}
	}
	return Filter{pattern: pattern, tokens: toks, tail: tail}, nil
}

// MustCompile is [Compile] that panics on error, for patterns known valid.
func MustCompile(pattern string) Filter {
	f, err := Compile(pattern)
	if err != nil {
		panic(err)
	}
	return f
}

// Pattern returns the pattern string the Filter was compiled from.
func (f Filter) Pattern() string { return f.pattern }

// MatchSubject reports whether the filter matches a subject string. On the hot
// path prefer [Filter.Match], splitting the subject once per publish.
func (f Filter) MatchSubject(subject string) bool {
	return f.Match(strings.Split(subject, Separator))
}

// Match reports whether the filter matches a subject already split into tokens.
func (f Filter) Match(subjectTokens []string) bool {
	for i, tok := range f.tokens {
		switch tok {
		case TailWildcard:
			return i < len(subjectTokens) // '>' must consume at least one token
		case TokenWildcard:
			if i >= len(subjectTokens) {
				return false
			}
		default:
			if i >= len(subjectTokens) || tok != subjectTokens[i] {
				return false
			}
		}
	}
	return len(f.tokens) == len(subjectTokens)
}
