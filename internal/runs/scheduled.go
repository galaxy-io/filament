package runs

import (
	"context"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/internal/compile"
	scheduledomain "github.com/galaxy-io/filament/internal/schedule"
)

// ReconcileScheduled re-derives a schedule's pre-created RunScheduled rows: it
// drops the pending set and, when the schedule will fire again, compiles that
// occurrence and pre-creates one row per route, so upcoming work is visible
// before the fire. It is the single owner of RunScheduled bookkeeping, called
// from the API on schedule saves and from the scheduler around fires and
// skips — the two sides converge because the occurrence token yields the same
// run ids the fire derives, which is what lets promotion find these rows.
func ReconcileScheduled(ctx context.Context, ds filament.DataStore, c *compile.Compiler, st filament.ScheduleState) error {
	if err := DropScheduled(ctx, ds, st.ID); err != nil {
		return err
	}
	if !st.Enabled || st.NextFire == nil {
		return nil
	}
	token := scheduledomain.OccurrenceToken(st.ID, *st.NextFire)
	compiled, err := c.Compile(ctx, st.Spec.PipelineID, token, filament.RunOptions{}, st.ID, filament.WorkerConfiguration{})
	if err != nil {
		return err
	}
	for _, cr := range compiled {
		// The fire path recompiles without a fire time, so this pre-create is
		// where scheduled_at comes from; promotion preserves it.
		cr.Submission.Request.ScheduledFor = *st.NextFire
		if _, err := Schedule(ctx, ds, cr.Submission.Request); err != nil {
			return err
		}
	}
	return nil
}

// DropScheduled deletes every pending RunScheduled row for the schedule.
func DropScheduled(ctx context.Context, ds filament.DataStore, id filament.ScheduleID) error {
	pending, _, err := ds.ListRuns(ctx, filament.RunFilter{
		Schedule: id,
		Status:   []filament.RunStatus{filament.RunScheduled},
	})
	if err != nil {
		return err
	}
	for _, r := range pending {
		if err := ds.DeleteRun(ctx, r.Run); err != nil {
			return err
		}
	}
	return nil
}
