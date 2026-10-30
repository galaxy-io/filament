package sinkstate

import (
	"context"
	"errors"
)

// SingleWrite is used by connectors whose capabilities only admit bounded runs.
// It prevents accidentally reusing run-scoped staging names for another write.
// Lifecycle methods are serialized by the caller.
type SingleWrite struct{ begun bool }

// Begin admits exactly one finite write for this sink instance.
func (s *SingleWrite) Begin(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.begun {
		return errors.New("sink: repeated writes are not supported by this connector")
	}
	s.begun = true
	return nil
}

// SingleWriteOnly identifies a lifecycle that cannot repeat write boundaries.
func (*SingleWrite) SingleWriteOnly() {}
