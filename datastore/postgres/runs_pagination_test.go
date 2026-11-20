package postgres_test

import (
	"testing"

	"github.com/galaxy-io/filament"
)

func TestStore_ListRunsEmptyPagePreservesTotal(t *testing.T) {
	store := newTestStore(t)
	for _, run := range []filament.RunState{
		{Run: runSortCompleted, Tenant: tenantA, Status: filament.RunCompleted},
		{Run: runSortCancelled, Tenant: tenantA, Status: filament.RunCompleted},
		{Run: runSortRunning, Tenant: tenantA, Status: filament.RunRunning},
	} {
		if err := store.SaveRun(t.Context(), run); err != nil {
			t.Fatal(err)
		}
	}
	for _, offset := range []int{0, 2} {
		runs, total, err := store.ListRuns(t.Context(), filament.RunFilter{Tenant: tenantA, Limit: 2, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		if len(runs) != min(2, 3-offset) || total != 3 {
			t.Fatalf("nonempty page at %d = %d runs, total %d", offset, len(runs), total)
		}
	}
	for _, tc := range []struct {
		name   string
		filter filament.RunFilter
		want   int
	}{
		{"end", filament.RunFilter{Tenant: tenantA, Limit: 2, Offset: 3}, 3},
		{"beyond", filament.RunFilter{Tenant: tenantA, Limit: 2, Offset: 100}, 3},
		{"unlimited", filament.RunFilter{Tenant: tenantA, Offset: 3}, 3},
		{"status", filament.RunFilter{Tenant: tenantA, Status: []filament.RunStatus{filament.RunCompleted}, Limit: 2, Offset: 2}, 2},
		{"search", filament.RunFilter{Tenant: tenantA, Search: runSortCompleted, Limit: 2, Offset: 1}, 1},
		{"no matches", filament.RunFilter{Tenant: tenantA, Status: []filament.RunStatus{filament.RunFailed}, Limit: 2, Offset: 100}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runs, total, err := store.ListRuns(t.Context(), tc.filter)
			if err != nil {
				t.Fatal(err)
			}
			if len(runs) != 0 || total != tc.want {
				t.Fatalf("ListRuns = %d runs, total %d; want 0 runs, total %d", len(runs), total, tc.want)
			}
		})
	}
}
