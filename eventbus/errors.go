package eventbus

import "errors"

var (
	// ErrBusClosed is returned by Publish/Subscribe after a bus is closed.
	ErrBusClosed = errors.New("eventbus: bus closed")

	// ErrInvalidToken is returned when a string can't be used as a single
	// subject token because it is empty or contains a separator, wildcard, or
	// whitespace — any of which would corrupt routing.
	ErrInvalidToken = errors.New("eventbus: invalid subject token")

	// ErrBadPattern is returned when a subscription pattern is malformed (e.g.
	// '>' not in final position, or an empty pattern).
	ErrBadPattern = errors.New("eventbus: malformed subject pattern")
)
