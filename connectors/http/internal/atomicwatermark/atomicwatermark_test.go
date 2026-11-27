package atomicwatermark

import "testing"

func TestNumericLessExactIntegers(t *testing.T) {
	n := Numeric{}
	cases := []struct {
		a, b string
		want bool
	}{
		// Neighbours past 2^53, where float64 rounds both sides together.
		{"9007199254740992", "9007199254740993", true},
		{"9007199254740993", "9007199254740992", false},
		{"9007199254740993", "9007199254740993", false},
		// int64 extremes.
		{"9223372036854775806", "9223372036854775807", true},
		{"-9223372036854775808", "-9223372036854775807", true},
		// Past int64 entirely.
		{"18446744073709551615", "18446744073709551616", true},
		// Ordinary integers keep working.
		{"9", "100", true},
		{"100", "9", false},
		{"-5", "3", true},
		{"007", "7", false},
		// Decimals still compare through the float path.
		{"1.5", "1.6", true},
		{"2", "1.999", false},
		{"0.1", "0.10", false},
	}
	for _, c := range cases {
		got, err := n.Less(c.a, c.b)
		if err != nil {
			t.Errorf("Less(%q, %q): %v", c.a, c.b, err)
			continue
		}
		if got != c.want {
			t.Errorf("Less(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
	if _, err := n.Less("abc", "1"); err == nil {
		t.Error("Less(abc, 1): want an error, got none")
	}
}

func TestNumericWatermarkAdvancesPast2To53(t *testing.T) {
	w, err := New(Numeric{}, "9007199254740992")
	if err != nil {
		t.Fatal(err)
	}
	adv, err := w.Observe("9007199254740993")
	if err != nil {
		t.Fatal(err)
	}
	if !adv || w.Current() != "9007199254740993" {
		t.Errorf("watermark did not advance: adv=%v current=%s", adv, w.Current())
	}
	// A repeat of the same value does not advance.
	adv, err = w.Observe("9007199254740993")
	if err != nil {
		t.Fatal(err)
	}
	if adv {
		t.Error("observing the current value reported an advance")
	}
}

func TestTimeLessAcceptsColonLessOffset(t *testing.T) {
	var cmp Time
	less, err := cmp.Less("2026-11-15T01:09:44.630+0000", "2026-11-15T01:09:45.630+00:00")
	if err != nil {
		t.Fatalf("Less = %v, want no error", err)
	}
	if !less {
		t.Errorf("Less = false, want true for an earlier colon-less offset")
	}
	less, err = cmp.Less("2026-11-15T01:09:45.630+00:00", "2026-11-15T01:09:44.630+0000")
	if err != nil {
		t.Fatalf("Less = %v, want no error", err)
	}
	if less {
		t.Errorf("Less = true, want false for a later value")
	}
	if _, err := cmp.Less("not a timestamp", "2026-11-15T01:09:44.630+00:00"); err == nil {
		t.Errorf("Less = no error, want an error for an unparseable value")
	}
}
