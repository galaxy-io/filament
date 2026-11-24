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
	"strings"
	"time"
)

// Layouts are the timestamp spellings tried in order. RFC3339Nano first so
// the common case parses without trying the rest. RFC1123Z is last: Twilio
// emits RFC 2822 dates (for example "Fri, 18 Oct 2019 17:00:00 +0000"),
// and no ISO spelling can match that shape, so existing parsing cannot
// change.
var Layouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999-0700",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02",
	time.RFC1123Z,
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
