package memory

import (
	"context"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

func TestDeleteScheduleReapsScheduledRuns(t *testing.T) {
	ctx := context.Background()
	store := New()
	if _, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: "p-1", TenantId: "t-1", Name: "Pipeline"}); err != nil {
		t.Fatal(err)
	}
	fire := time.Now().Add(time.Hour)
	if err := store.SaveSchedule(ctx, filament.ScheduleState{
		ID:       "s-1",
		Spec:     filament.ScheduleSpec{Tenant: "t-1", PipelineID: "p-1", Cron: "0 * * * *", Timezone: "UTC"},
		Enabled:  true,
		NextFire: &fire,
	}); err != nil {
		t.Fatal(err)
	}
	for _, run := range []filament.RunState{
		{Run: "r-pending", Tenant: "t-1", Status: filament.RunScheduled, ScheduleID: "s-1", ScheduledAt: fire},
		{Run: "r-promoted", Tenant: "t-1", Status: filament.RunRequested, ScheduleID: "s-1", RequestedAt: time.Now()},
	} {
		if err := store.CreateRun(ctx, run); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.DeleteSchedule(ctx, "t-1", "s-1"); err != nil {
		t.Fatal(err)
	}

	pending, _, err := store.ListRuns(ctx, filament.RunFilter{Tenant: "t-1", Status: []filament.RunStatus{filament.RunScheduled}})
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected pending scheduled runs reaped on schedule delete, got %+v", pending)
	}
	promoted, err := store.LoadRun(ctx, "t-1", "r-promoted")
	if err != nil {
		t.Fatal(err)
	}
	if promoted.Status != filament.RunRequested {
		t.Fatalf("expected promoted run to survive schedule delete, got %+v", promoted)
	}
}
