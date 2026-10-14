// Package schema compares complete schemas and evaluates migration rules.
// Connectors own inspection, locking, DDL, and verification.
package schema

import (
	"errors"
	"fmt"
	"strings"

	"github.com/galaxy-io/filament/rowmodel"
)

// Policy selects the set of automatically permitted changes.
type Policy string

// Supported migration policies.
const (
	Strict      Policy = "strict"
	NullableAdd Policy = "nullable_add"
)

// Validate rejects unknown policies, including an unset policy.
func (p Policy) Validate() error {
	if p != Strict && p != NullableAdd {
		return fmt.Errorf("unknown schema policy %q", p)
	}
	return nil
}

// ErrBlocked identifies drift requiring intervention before another attempt.
var ErrBlocked = errors.New("schema_blocked")

// BlockError is actionable drift information, without row values. A block never
// authorizes acknowledgement, progress advancement, or destructive DDL.
type BlockError struct {
	Resource string
	Reasons  []string
}

func (e *BlockError) Error() string {
	return fmt.Sprintf("%s: %s: %s", ErrBlocked, e.Resource, strings.Join(e.Reasons, "; "))
}
func (e *BlockError) Unwrap() error { return ErrBlocked }

// Check rejects the entire diff if any change is disallowed. A connector must
// additionally verify that it can safely implement every permitted change.
// Column order is meaningful here. Name-based sinks may normalize it first.
func (p Policy) Check(resource string, d Diff) error {
	if err := p.Validate(); err != nil {
		return err
	}
	var reasons []string
	for _, c := range d.Changes {
		if p == NullableAdd && c.Kind == AddField && c.After != nil && c.After.Nullable && c.After.Logical != rowmodel.LogicalUnknown {
			continue
		}
		reasons = append(reasons, fmt.Sprintf("%s %q is not allowed by %s", c.Kind, c.Field, p))
	}
	if len(reasons) > 0 {
		return &BlockError{Resource: resource, Reasons: reasons}
	}
	return nil
}
