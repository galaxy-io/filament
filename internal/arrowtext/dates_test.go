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
