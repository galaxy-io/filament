package arrowtext

// Calendar arithmetic and ISO 8601 text for the Arrow date and time storage
// units: days since 1970-01-01 (Date32) and microseconds since the Unix epoch
// (Timestamp) or midnight (Time64). Proleptic Gregorian; year 0 is 1 BC.

import (
	"errors"
	"strconv"
)

// Microseconds per unit.
const (
	MicrosPerSecond = int64(1_000_000)
	MicrosPerMinute = 60 * MicrosPerSecond
	MicrosPerHour   = 60 * MicrosPerMinute
	MicrosPerDay    = 24 * MicrosPerHour
)

// DaysToDate converts days since 1970-01-01 to a calendar date (Hinnant's
// civil_from_days). Year <= 0 means BC (0 is 1 BC).
func DaysToDate(days int64) (y int64, m, d int) {
	z := days + 719468
	era := FloorDiv(z, 146097)
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d = int(doy - (153*mp+2)/5 + 1)
	if mp < 10 {
		m = int(mp) + 3
	} else {
		m = int(mp) - 9
	}
	y = yoe + era*400
	if m <= 2 {
		y++
	}
	return y, m, d
}

// DateToDays converts a calendar date to days since 1970-01-01.
func DateToDays(y int64, m, d int) int64 {
	if m <= 2 {
		y--
	}
	era := FloorDiv(y, 400)
	yoe := y - era*400
	mp := int64(m+9) % 12
	doy := (153*mp+2)/5 + int64(d) - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

// FloorDiv is integer division rounding toward negative infinity (b > 0).
func FloorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && a < 0 {
		q--
	}
	return q
}

// AppendDate32 appends "YYYY-MM-DD" for Arrow Date32 days since the epoch, with a " BC" suffix
// for years before 1 AD.
func AppendDate32(dst []byte, days int32) []byte {
	y, m, d := DaysToDate(int64(days))
	bc := y <= 0
	if bc {
		y = 1 - y
	}
	dst = AppendPadded(dst, y, 4)
	dst = append(dst, '-')
	dst = AppendPadded(dst, int64(m), 2)
	dst = append(dst, '-')
	dst = AppendPadded(dst, int64(d), 2)
	if bc {
		dst = append(dst, " BC"...)
	}
	return dst
}

// AppendTimestamp appends "YYYY-MM-DD<sep>HH:MM:SS[.ffffff][ BC]" for
// microseconds since the epoch; sep is ' ' (SQL) or 'T' (ISO 8601).
func AppendTimestamp(dst []byte, us int64, sep byte) []byte {
	days := FloorDiv(us, MicrosPerDay)
	y, m, d := DaysToDate(days)
	bc := y <= 0
	if bc {
		y = 1 - y
	}
	dst = AppendPadded(dst, y, 4)
	dst = append(dst, '-')
	dst = AppendPadded(dst, int64(m), 2)
	dst = append(dst, '-')
	dst = AppendPadded(dst, int64(d), 2)
	dst = append(dst, sep)
	dst = AppendTimeOfDay(dst, us-days*MicrosPerDay)
	if bc {
		dst = append(dst, " BC"...)
	}
	return dst
}

// AppendTimeOfDay appends "HH:MM:SS" for microseconds since midnight, with the
// fraction, when non-zero, trimmed of trailing zeros.
func AppendTimeOfDay(dst []byte, us int64) []byte {
	dst = AppendPadded(dst, us/MicrosPerHour, 2)
	dst = append(dst, ':')
	dst = AppendPadded(dst, us%MicrosPerHour/MicrosPerMinute, 2)
	dst = append(dst, ':')
	dst = AppendPadded(dst, us%MicrosPerMinute/MicrosPerSecond, 2)
	if frac := us % MicrosPerSecond; frac > 0 {
		dst = append(dst, '.')
		dst = AppendPadded(dst, frac, 6)
		for dst[len(dst)-1] == '0' {
			dst = dst[:len(dst)-1]
		}
	}
	return dst
}

// AppendPadded appends v left-padded with zeros to width digits.
func AppendPadded(dst []byte, v int64, width int) []byte {
	var buf [20]byte
	b := strconv.AppendInt(buf[:0], v, 10)
	for i := len(b); i < width; i++ {
		dst = append(dst, '0')
	}
	return append(dst, b...)
}

// ErrBadDate reports text that is not a date or time of day.
var ErrBadDate = errors.New("invalid date")

// ReadDate reads "YYYY-MM-DD" and returns days since the Unix epoch and the
// unread tail.
func ReadDate(text []byte) (days int64, rest []byte, err error) {
	y, rest, ok := digits(text, 4)
	if !ok || len(rest) == 0 || rest[0] != '-' {
		return 0, nil, ErrBadDate
	}
	m, rest, ok := digits(rest[1:], 2)
	if !ok || len(rest) == 0 || rest[0] != '-' {
		return 0, nil, ErrBadDate
	}
	d, rest, ok := digits(rest[1:], 2)
	if !ok {
		return 0, nil, ErrBadDate
	}
	return DateToDays(y, int(m), int(d)), rest, nil
}

// ReadTimeOfDay reads "HH:MM:SS[.ffffff]" (two or three hour digits) as
// microseconds and returns the unread tail.
func ReadTimeOfDay(text []byte) (us int64, rest []byte, err error) {
	n := 0
	for n < len(text) && n < 3 && text[n] >= '0' && text[n] <= '9' {
		n++
	}
	h, rest, ok := digits(text, n)
	if n == 0 || !ok || len(rest) == 0 || rest[0] != ':' {
		return 0, nil, ErrBadDate
	}
	m, rest, ok := digits(rest[1:], 2)
	if !ok || len(rest) == 0 || rest[0] != ':' {
		return 0, nil, ErrBadDate
	}
	s, rest, ok := digits(rest[1:], 2)
	if !ok {
		return 0, nil, ErrBadDate
	}
	us = h*MicrosPerHour + m*MicrosPerMinute + s*MicrosPerSecond
	if len(rest) > 0 && rest[0] == '.' {
		rest = rest[1:]
		var frac int64
		k := 0
		for k < len(rest) && rest[k] >= '0' && rest[k] <= '9' {
			if k < 6 {
				frac = frac*10 + int64(rest[k]-'0')
			}
			k++
		}
		if k == 0 {
			return 0, nil, ErrBadDate
		}
		for i := k; i < 6; i++ {
			frac *= 10
		}
		us += frac
		rest = rest[k:]
	}
	return us, rest, nil
}

// Digits reads exactly n decimal digits and returns the value and the unread tail.
func Digits(text []byte, n int) (v int64, rest []byte, ok bool) { return digits(text, n) }

func digits(text []byte, n int) (int64, []byte, bool) {
	if len(text) < n {
		return 0, nil, false
	}
	var v int64
	for i := range n {
		c := text[i]
		if c < '0' || c > '9' {
			return 0, nil, false
		}
		v = v*10 + int64(c-'0')
	}
	return v, text[n:], true
}
