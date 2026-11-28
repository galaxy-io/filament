// Package timestamps parses the ISO 8601 spellings APIs commonly emit for
// timestamp values, and normalizes them to RFC3339 for storage.
//
// Jira Cloud, for example, emits created and updated as
// 2026-11-15T01:09:44.630+0000, a colon-less numeric offset that strict
// RFC3339 parsing rejects. Timestamp columns and incremental cursors share
// these layouts, so the two paths cannot drift apart again.
package timestamps

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Layouts are the timestamp spellings tried in order. RFC3339Nano first so
// the common case parses without trying the rest.
var Layouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999-0700",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02",
}

// Parse reads s as an instant, trying each layout in order.
func Parse(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range Layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("%q is not a timestamp", s)
}

// Normalize parses s and renders it as RFC3339Nano, the canonical form
// stored in checkpoints and compared by the time comparator.
func Normalize(s string) (string, error) {
	t, err := Parse(s)
	if err != nil {
		return "", err
	}
	return t.Format(time.RFC3339Nano), nil
}

// epochMillisAbove separates the two epoch units numbers arrive in. Unix
// seconds stay below it until the year 33658; milliseconds pass it from
// September 2001.
const epochMillisAbove = 1e12

// EpochMicros reads a Unix epoch number, in seconds or milliseconds, integer
// or fractional, as microseconds since the epoch. Values above 1e12 are
// milliseconds. timestamptz columns and time cursors both read JSON numbers
// with this rule.
func EpochMicros(f float64) int64 {
	if f > epochMillisAbove {
		return int64(math.Round(f * 1000))
	}
	return int64(math.Round(f * 1_000_000))
}

// NormalizeEpoch renders a Unix epoch number, read with EpochMicros, as the
// RFC3339Nano form Normalize produces.
func NormalizeEpoch(f float64) string {
	return time.UnixMicro(EpochMicros(f)).UTC().Format(time.RFC3339Nano)
}
