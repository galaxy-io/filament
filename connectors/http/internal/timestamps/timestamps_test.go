package timestamps

import "testing"

func TestParseAcceptsCommonSpellings(t *testing.T) {
	cases := []string{
		"2026-11-15T01:09:44.630Z",
		"2026-11-15T01:09:44.630+00:00",
		"2026-11-15T01:09:44.630+0000",
		"2026-11-13T01:09:44.630+0500",
		"2026-11-15T01:09:44+00:00",
		"2026-11-15 01:09:44.630+00:00",
		"2026-11-15",
	}
	for _, c := range cases {
		if _, err := Parse(c); err != nil {
			t.Errorf("Parse(%q) = %v, want no error", c, err)
		}
	}
	if _, err := Parse("not a timestamp"); err == nil {
		t.Errorf("Parse(%q) = no error, want an error", "not a timestamp")
	}
}

func TestNormalizeRendersRFC3339(t *testing.T) {
	got, err := Normalize("2026-11-15T01:09:44.630+0000")
	if err != nil {
		t.Fatalf("Normalize = %v, want no error", err)
	}
	if want := "2026-11-15T01:09:44.63Z"; got != want {
		t.Errorf("Normalize = %q, want %q", got, want)
	}
	// A value already in RFC3339 is unchanged.
	got, err = Normalize("2024-11-07T12:00:00Z")
	if err != nil {
		t.Fatalf("Normalize = %v, want no error", err)
	}
	if want := "2024-11-07T12:00:00Z"; got != want {
		t.Errorf("Normalize = %q, want %q", got, want)
	}
}

func TestEpochMicrosUnits(t *testing.T) {
	for _, tc := range []struct {
		in   float64
		want string
	}{
		{1795822894, "2026-11-27T23:41:34Z"},
		{1795822894.25, "2026-11-27T23:41:34.25Z"},
		{1795822894123, "2026-11-27T23:41:34.123Z"},
		{1e12, "33658-09-27T01:46:40Z"},
		{0, "1970-01-01T00:00:00Z"},
	} {
		if got := NormalizeEpoch(tc.in); got != tc.want {
			t.Errorf("NormalizeEpoch(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
