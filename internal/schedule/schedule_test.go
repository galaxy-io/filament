package schedule

import (
	"testing"
	"time"

	"github.com/galaxy-io/filament"
)

// A daily schedule at a fixed local time fires once on both daylight saving
// change days in New York.
func TestNextFireDaylightSaving(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cron  string
		after time.Time
		want  time.Time
	}{
		// After the 01:30 EDT fire on Nov 1, the next fire is 01:30 EST on
		// Nov 2, not the repeated 01:30 EST on Nov 1.
		{"fall back", "30 1 * * *", time.Date(2026, 11, 1, 5, 30, 5, 0, time.UTC), time.Date(2026, 11, 2, 6, 30, 0, 0, time.UTC)},
		// After the 02:30 EST fire on Mar 7, Mar 8 has no 02:30, so the next
		// fire is 03:00 EDT on Mar 8.
		{"spring forward", "30 2 * * *", time.Date(2026, 3, 7, 7, 30, 5, 0, time.UTC), time.Date(2026, 3, 8, 7, 0, 0, 0, time.UTC)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := filament.ScheduleSpec{
				PipelineID: "p",
				Tenant:     filament.DefaultTenantID,
				Cron:       tc.cron,
				Timezone:   "America/New_York",
			}
			next, err := NextFire(spec, tc.after)
			if err != nil {
				t.Fatal(err)
			}
			if !next.Equal(tc.want) {
				t.Fatalf("NextFire(%s) = %s; want %s", tc.after, next, tc.want)
			}
		})
	}
}
