package eventbus

import (
	"fmt"
	"strings"
	"unicode"
)

// Subject grammar primitives shared by every domain. A subject is dot-separated
// tokens; the bus is agnostic to how many there are or what they mean.
const (
	Separator     = "."
	TokenWildcard = "*" // matches exactly one token
	TailWildcard  = ">" // matches one or more trailing tokens (final token only)
)

// ValidToken reports whether s is usable as a single subject token: non-empty
// and free of the separator, wildcards, and whitespace. It returns a wrapped
// [ErrInvalidToken] naming the first problem, or nil.
func ValidToken(s string) error {
	if s == "" {
		return fmt.Errorf("%w: empty", ErrInvalidToken)
	}
	for _, r := range s {
		switch {
		case r == '.':
			return fmt.Errorf("%w: %q contains separator '.'", ErrInvalidToken, s)
		case r == '*' || r == '>':
			return fmt.Errorf("%w: %q contains wildcard %q", ErrInvalidToken, s, string(r))
		case unicode.IsSpace(r):
			return fmt.Errorf("%w: %q contains whitespace", ErrInvalidToken, s)
		}
	}
	return nil
}

// IsValidToken is the boolean form of ValidToken.
func IsValidToken(s string) bool { return ValidToken(s) == nil }

// SubjectMatch reports whether a NATS-style pattern matches a concrete subject.
// It is the reference matcher; [Filter] is the compiled form for the hot path.
func SubjectMatch(pattern, subject string) bool {
	p := strings.Split(pattern, Separator)
	s := strings.Split(subject, Separator)
	for i, tok := range p {
		switch tok {
		case TailWildcard:
			return i < len(s) // '>' must consume at least one remaining token
		case TokenWildcard:
			if i >= len(s) {
				return false
			}
		default:
			if i >= len(s) || tok != s[i] {
				return false
			}
		}
	}
	return len(p) == len(s)
}
