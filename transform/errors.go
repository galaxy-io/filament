package transform

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalid is the sentinel every compile-time failure wraps.
var ErrInvalid = errors.New("transform: invalid definition")

// Errors collects every problem a compile pass finds, so an author sees all
// of them at once rather than one per attempt.
type Errors struct {
	Issues []Issue
}

// Issue is one problem and the path in the definition where it sits.
type Issue struct {
	Path    string // e.g. resources["users"].steps[2].set.email
	Message string
}

// Error renders one issue inline and several as a list, one per line.
func (e *Errors) Error() string {
	if len(e.Issues) == 1 {
		return fmt.Sprintf("transform invalid: %s: %s", e.Issues[0].Path, e.Issues[0].Message)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "transform invalid (%d issues):", len(e.Issues))
	for _, iss := range e.Issues {
		fmt.Fprintf(&b, "\n  - %s: %s", iss.Path, iss.Message)
	}
	return b.String()
}

// Unwrap makes errors.Is(err, ErrInvalid) true for every Errors value.
func (e *Errors) Unwrap() error { return ErrInvalid }

// addf records one issue at path.
func (e *Errors) addf(path, format string, args ...any) {
	e.Issues = append(e.Issues, Issue{Path: path, Message: fmt.Sprintf(format, args...)})
}

// asError returns e as an error, or nil when nothing was recorded.
func (e *Errors) asError() error {
	if len(e.Issues) == 0 {
		return nil
	}
	return e
}
