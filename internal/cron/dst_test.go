package cron

import (
	"testing"
	"time"
)

// 2026 changes: New York skips 02:00-02:59 on Mar 8 and repeats 01:00-01:59 on
// Nov 1. London skips 01:00-01:59 on Mar 29 and repeats 01:00-01:59 on Oct 25.
func TestNextDaylightSaving(t *testing.T) {
	for _, tc := range []struct {
		name  string
		zone  string
		spec  string
		after string // UTC
		want  string // UTC
	}{
		// Fixed local time: a repeated time fires once, a skipped time fires
		// right after the gap.
		{"new york fall back first 01:30", "America/New_York", "30 1 * * *", "2026-11-01T04:00:00Z", "2026-11-01T05:30:00Z"},
		{"new york fall back no second 01:30", "America/New_York", "30 1 * * *", "2026-11-01T05:30:05Z", "2026-11-02T06:30:00Z"},
		{"new york fall back inside repeated hour", "America/New_York", "30 1 * * *", "2026-11-01T06:10:00Z", "2026-11-02T06:30:00Z"},
		{"new york spring forward fires after gap", "America/New_York", "30 2 * * *", "2026-03-07T07:30:05Z", "2026-03-08T07:00:00Z"},
		{"new york spring forward next day", "America/New_York", "30 2 * * *", "2026-03-08T07:00:00Z", "2026-03-09T06:30:00Z"},
		{"london fall back no second 01:30", "Europe/London", "30 1 * * *", "2026-10-25T00:30:05Z", "2026-10-26T01:30:00Z"},
		{"london spring forward fires after gap", "Europe/London", "30 1 * * *", "2026-03-28T01:30:05Z", "2026-03-29T01:00:00Z"},
		{"new york hour range skips repeated 01:00", "America/New_York", "0 1-3 * * *", "2026-11-01T05:00:05Z", "2026-11-01T07:00:00Z"},
		{"new york minute list skips repeated 01:00", "America/New_York", "0,30 1 * * *", "2026-11-01T05:30:05Z", "2026-11-02T06:00:00Z"},
		{"new york full minute range skips repeated hour", "America/New_York", "0-59 1 * * *", "2026-11-01T05:59:05Z", "2026-11-02T06:00:00Z"},
		{"new york full hour range skips repeated 01:30", "America/New_York", "30 0-23 * * *", "2026-11-01T05:30:05Z", "2026-11-01T07:30:00Z"},
		{"london hour list fires at end of gap", "Europe/London", "0 1,2 * * *", "2026-03-29T00:00:05Z", "2026-03-29T01:00:00Z"},
		{"london hour list fires once on gap day", "Europe/London", "0 1,2 * * *", "2026-03-29T01:00:00Z", "2026-03-30T00:00:00Z"},

		// Minute or hour starts with '*': follows real time.
		{"new york step minute fires in repeated hour", "America/New_York", "*/15 1 * * *", "2026-11-01T05:45:05Z", "2026-11-01T06:00:00Z"},
		{"new york hourly fires in repeated hour", "America/New_York", "30 * * * *", "2026-11-01T05:30:05Z", "2026-11-01T06:30:00Z"},
		{"new york step hour skips gap", "America/New_York", "0 */2 * * *", "2026-03-08T05:00:05Z", "2026-03-08T08:00:00Z"},
		{"london every minute skips gap", "Europe/London", "* 1 * * *", "2026-03-28T01:59:05Z", "2026-03-30T00:00:00Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loc, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			s, err := Parse(tc.spec)
			if err != nil {
				t.Fatal(err)
			}
			after, err := time.Parse(time.RFC3339, tc.after)
			if err != nil {
				t.Fatal(err)
			}
			want, err := time.Parse(time.RFC3339, tc.want)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := s.Next(after.In(loc))
			if !ok || !got.Equal(want) {
				t.Fatalf("Next(%s) = %s, %v; want %s", after.In(loc), got, ok, want.In(loc))
			}
		})
	}
}

// A fixed local time fires exactly once on every day around both changes.
func TestNextDaylightSavingOncePerDay(t *testing.T) {
	for _, tc := range []struct {
		zone string
		spec string
		from string // UTC
		days int
	}{
		{"America/New_York", "30 1 * * *", "2026-10-30T04:00:00Z", 5},
		{"America/New_York", "30 2 * * *", "2026-03-06T05:00:00Z", 5},
		{"Europe/London", "30 1 * * *", "2026-10-23T00:00:00Z", 5},
		{"Europe/London", "30 1 * * *", "2026-03-27T00:00:00Z", 5},
	} {
		t.Run(tc.zone+" "+tc.from, func(t *testing.T) {
			loc, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			s, err := Parse(tc.spec)
			if err != nil {
				t.Fatal(err)
			}
			from, err := time.Parse(time.RFC3339, tc.from)
			if err != nil {
				t.Fatal(err)
			}
			at := from.In(loc)
			seen := map[string]int{}
			for range tc.days {
				next, ok := s.Next(at)
				if !ok {
					t.Fatalf("Next(%s) found no fire", at)
				}
				seen[next.Format(time.DateOnly)]++
				at = next
			}
			first := from.In(loc)
			for d := range tc.days {
				day := time.Date(first.Year(), first.Month(), first.Day()+d, 0, 0, 0, 0, loc).Format(time.DateOnly)
				if seen[day] != 1 {
					t.Fatalf("%s fired %d times, want 1 (fires by day: %v)", day, seen[day], seen)
				}
			}
		})
	}
}
