package arrowtext

import (
	"errors"
	"testing"
)

func TestReadDateYearWidth(t *testing.T) {
	ok := map[string]int64{
		"2024-02-29":    DateToDays(2024, 2, 29),
		"10000-01-01":   DateToDays(10000, 1, 1),
		"5874897-12-31": DateToDays(5874897, 12, 31),
	}
	for in, want := range ok {
		got, rest, err := ReadDate([]byte(in + " tail"))
		if err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		if got != want || string(rest) != " tail" {
			t.Errorf("%s = %d, %q; want %d, %q", in, got, rest, want, " tail")
		}
	}
	for _, in := range []string{"999-01-01", "12345678-01-01", "2024", ""} {
		if _, _, err := ReadDate([]byte(in)); !errors.Is(err, ErrBadDate) {
			t.Errorf("%q: err = %v, want ErrBadDate", in, err)
		}
	}
}

func TestReadDateRangeChecks(t *testing.T) {
	ok := []string{
		"2024-02-29", // leap year
		"2000-02-29", // divisible by 400
		"2023-02-28",
		"2024-01-31",
		"2024-04-30",
		"2024-12-31",
		"0001-01-01",
	}
	for _, in := range ok {
		if _, _, err := ReadDate([]byte(in)); err != nil {
			t.Errorf("%q: unexpected err %v", in, err)
		}
	}
	bad := []string{
		"2024-02-30",
		"2024-02-31",
		"2023-02-29", // not a leap year
		"1900-02-29", // divisible by 100 but not 400
		"2024-04-31",
		"2024-13-01",
		"2024-00-15",
		"2024-01-00",
		"2024-01-32",
	}
	for _, in := range bad {
		if _, _, err := ReadDate([]byte(in)); !errors.Is(err, ErrBadDate) {
			t.Errorf("%q: err = %v, want ErrBadDate", in, err)
		}
	}
}

func TestReadTimeOfDayRangeChecks(t *testing.T) {
	ok := map[string]int64{
		"00:00:00":        0,
		"23:59:59":        MicrosPerDay - MicrosPerSecond,
		"23:59:59.999999": MicrosPerDay - 1,
		"12:00:00.5":      12*MicrosPerHour + MicrosPerSecond/2,
	}
	for in, want := range ok {
		got, rest, err := ReadTimeOfDay([]byte(in))
		if err != nil {
			t.Errorf("%q: unexpected err %v", in, err)
			continue
		}
		if got != want || len(rest) != 0 {
			t.Errorf("%q = %d, %q; want %d, %q", in, got, rest, want, "")
		}
	}
	bad := []string{
		"12:60:30",
		"12:00:60",
		"23:59:60",
		"24:00:00",
		"25:00:00",
		"99:00:00",
	}
	for _, in := range bad {
		if _, _, err := ReadTimeOfDay([]byte(in)); !errors.Is(err, ErrBadDate) {
			t.Errorf("%q: err = %v, want ErrBadDate", in, err)
		}
	}
}
