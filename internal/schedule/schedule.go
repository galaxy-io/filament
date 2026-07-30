// Package schedule contains schedule-domain validation and timing calculations.
package schedule

import (
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/internal/cron"
)

// NextFire validates spec and returns its first cron occurrence after the
// supplied instant.
func NextFire(spec filament.ScheduleSpec, after time.Time) (*time.Time, error) {
	if spec.PipelineID == "" {
		return nil, fmt.Errorf("schedule: pipeline id is required")
	}
	if err := spec.Tenant.Valid(); err != nil {
		return nil, fmt.Errorf("schedule: tenant %w", err)
	}
	sched, err := cron.Parse(spec.Cron)
	if err != nil {
		return nil, err
	}
	loc := time.UTC
	if spec.Timezone != "" {
		loc, err = time.LoadLocation(spec.Timezone)
		if err != nil {
			return nil, fmt.Errorf("schedule: timezone %q: %w", spec.Timezone, err)
		}
	}
	next, ok := sched.Next(after.In(loc))
	if !ok {
		return nil, fmt.Errorf("schedule: cron %q has no next fire", spec.Cron)
	}
	return &next, nil
}
