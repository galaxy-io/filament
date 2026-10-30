package schema

import "context"

// Detection describes observed drift, not a successful migration. It contains
// field names and change categories, never source values or native DDL.
type Detection struct {
	Side    string        `json:"side"`
	Policy  Policy        `json:"policy"`
	Changes []FieldChange `json:"changes"`
}

// FieldChange is the diagnostic projection of a structural difference.
type FieldChange struct {
	Kind  ChangeKind `json:"kind"`
	Field string     `json:"field,omitempty"`
}

type observerKey struct{}

// WithObserver installs a synchronous diagnostic callback. Observers must be
// safe for concurrent resource writers. They cannot authorize schema changes.
func WithObserver(ctx context.Context, observer func(string, Detection)) context.Context {
	return context.WithValue(ctx, observerKey{}, observer)
}

// Observe reports only nonempty diffs. The projection cannot mutate the plan.
func Observe(ctx context.Context, side, resource string, policy Policy, diff Diff) {
	observer, _ := ctx.Value(observerKey{}).(func(string, Detection))
	if observer == nil || len(diff.Changes) == 0 {
		return
	}
	d := Detection{Side: side, Policy: policy}
	for _, change := range diff.Changes {
		d.Changes = append(d.Changes, FieldChange{Kind: change.Kind, Field: change.Field})
	}
	observer(resource, d)
}
