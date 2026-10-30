package schema

import (
	"context"
	"slices"
	"strings"

	"github.com/galaxy-io/filament/rowmodel"
)

// Target is an existing, protected destination table. The connector owns table
// creation, type normalization, locks and transaction lifetime. AddNullable must
// restore protection before returning if DDL releases the destination's locks.
type Target interface {
	Inspect(context.Context) (rowmodel.Schema, error)
	AddNullable(context.Context, []rowmodel.Field) error
}

// CompareNamed compares tables addressed by column name, ignoring physical order.
// Both models must already use the connector's canonical physical types.
func CompareNamed(actual, desired rowmodel.Schema) (Diff, error) {
	actual, desired = actual.Clone(), desired.Clone()
	less := func(a, b rowmodel.Field) int { return strings.Compare(a.Name, b.Name) }
	slices.SortFunc(actual.Fields, less)
	slices.SortFunc(desired.Fields, less)
	return Compare(actual, desired)
}

// Reconcile validates the entire diff before invoking DDL, then verifies it.
// Additions are passed together so connectors can use one ALTER statement.
// It never creates, truncates or commits. On error, committed DDL may remain on
// nontransactional destinations. A retry always inspects the actual table.
func Reconcile(ctx context.Context, target Target, desired rowmodel.Schema, policy Policy) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	actual, err := target.Inspect(ctx)
	if err != nil {
		return err
	}
	diff, err := CompareNamed(actual, desired)
	if err != nil {
		return err
	}
	Observe(ctx, "sink", desired.Resource, policy, diff)
	if err := policy.Check(desired.Resource, diff); err != nil {
		return err
	}
	if len(diff.Changes) == 0 {
		return nil
	}
	fields := make([]rowmodel.Field, 0, len(diff.Changes))
	for _, change := range diff.Changes {
		fields = append(fields, *change.After)
	}
	if err := target.AddNullable(ctx, fields); err != nil {
		return err
	}
	actual, err = target.Inspect(ctx)
	if err != nil {
		return err
	}
	diff, err = CompareNamed(actual, desired)
	if err != nil {
		return err
	}
	return Strict.Check(desired.Resource, diff)
}
