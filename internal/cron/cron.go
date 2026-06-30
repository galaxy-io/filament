// Package cron parses standard 5-field cron expressions and computes the next
// fire time, with no external dependencies. Fields are: minute (0–59), hour
// (0–23), day-of-month (1–31), month (1–12), day-of-week (0–6, Sunday=0; 7 also
// accepted for Sunday). Each field supports '*', lists (a,b), ranges (a-b), and
// steps (*/n or a-b/n). It deliberately omits names (JAN, MON), '?', 'L', 'W',
// and '#' — the scheduler only needs the common numeric grammar.
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule is a parsed cron expression as per-field bit sets.
type Schedule struct {
	minute uint64 // bits 0..59
	hour   uint64 // bits 0..23
	dom    uint64 // bits 1..31
	month  uint64 // bits 1..12
	dow    uint64 // bits 0..6 (Sunday=0)

	domRestricted bool // day-of-month field was not "*"
	dowRestricted bool // day-of-week field was not "*"
}

// maxSearch bounds Next's minute-by-minute scan at ~5 years — enough to reach the
// rarest valid schedule (e.g. Feb 29) while still terminating on an impossible one.
const maxSearch = 5 * 366 * 24 * 60

// Parse compiles a 5-field cron expression.
func Parse(expr string) (Schedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return Schedule{}, fmt.Errorf("cron: expected 5 fields, got %d in %q", len(fields), expr)
	}

	var s Schedule
	var err error
	if s.minute, err = parseField(fields[0], 0, 59); err != nil {
		return Schedule{}, fmt.Errorf("cron: minute: %w", err)
	}
	if s.hour, err = parseField(fields[1], 0, 23); err != nil {
		return Schedule{}, fmt.Errorf("cron: hour: %w", err)
	}
	if s.dom, err = parseField(fields[2], 1, 31); err != nil {
		return Schedule{}, fmt.Errorf("cron: day-of-month: %w", err)
	}
	if s.month, err = parseField(fields[3], 1, 12); err != nil {
		return Schedule{}, fmt.Errorf("cron: month: %w", err)
	}
	if s.dow, err = parseField(fields[4], 0, 7); err != nil {
		return Schedule{}, fmt.Errorf("cron: day-of-week: %w", err)
	}
	// Fold Sunday-as-7 into Sunday-as-0.
	if s.dow&(1<<7) != 0 {
		s.dow = (s.dow &^ (1 << 7)) | 1
	}

	s.domRestricted = fields[2] != "*"
	s.dowRestricted = fields[4] != "*"
	return s, nil
}

// Next returns the first fire time strictly after t, and whether one was found
// within the search bound. The result is in t's location.
func (s Schedule) Next(t time.Time) (time.Time, bool) {
	t = t.Truncate(time.Minute).Add(time.Minute)
	for range maxSearch {
		if s.matches(t) {
			return t, true
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}, false
}

// matches reports whether t satisfies every field. Day-of-month and day-of-week
// combine with cron's quirk: when both are restricted a day matches if *either*
// does; otherwise both must (and an unrestricted field's bits are all set, so the
// AND is a no-op).
func (s Schedule) matches(t time.Time) bool {
	if s.minute&(1<<uint(t.Minute())) == 0 {
		return false
	}
	if s.hour&(1<<uint(t.Hour())) == 0 {
		return false
	}
	if s.month&(1<<uint(int(t.Month()))) == 0 {
		return false
	}
	domMatch := s.dom&(1<<uint(t.Day())) != 0
	dowMatch := s.dow&(1<<uint(int(t.Weekday()))) != 0
	if s.domRestricted && s.dowRestricted {
		return domMatch || dowMatch
	}
	return domMatch && dowMatch
}

// parseField compiles one comma-separated field into a bit set.
func parseField(spec string, min, max int) (uint64, error) {
	var bits uint64
	for part := range strings.SplitSeq(spec, ",") {
		b, err := parsePart(part, min, max)
		if err != nil {
			return 0, err
		}
		bits |= b
	}
	return bits, nil
}

// parsePart compiles one term: '*', 'n', 'a-b', or any of those with a '/step'.
func parsePart(part string, min, max int) (uint64, error) {
	step := 1
	rng := part
	if i := strings.Index(part, "/"); i >= 0 {
		rng = part[:i]
		n, err := strconv.Atoi(part[i+1:])
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid step in %q", part)
		}
		step = n
	}

	var lo, hi int
	switch {
	case rng == "*":
		lo, hi = min, max
	case strings.ContainsRune(rng, '-'):
		ab := strings.SplitN(rng, "-", 2)
		a, err1 := strconv.Atoi(ab[0])
		b, err2 := strconv.Atoi(ab[1])
		if err1 != nil || err2 != nil {
			return 0, fmt.Errorf("invalid range %q", rng)
		}
		lo, hi = a, b
	default:
		n, err := strconv.Atoi(rng)
		if err != nil {
			return 0, fmt.Errorf("invalid value %q", rng)
		}
		lo, hi = n, n
	}

	if lo < min || hi > max || lo > hi {
		return 0, fmt.Errorf("value out of range [%d,%d] in %q", min, max, part)
	}

	var bits uint64
	for v := lo; v <= hi; v += step {
		bits |= 1 << uint(v)
	}
	return bits, nil
}
