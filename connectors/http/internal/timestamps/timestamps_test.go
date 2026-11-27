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
		// Twilio emits RFC 2822 dates, e.g. calls date_created and
		// messages date_sent. These must parse so timestamptz fields
		// and time-comparator cursors work on Twilio records.
		"Fri, 18 Oct 2019 17:00:00 +0000",
		"Fri, 18 Oct 2019 17:02:00 +0000",
		"Thu, 24 Aug 2023 05:01:45 +0000",
		"Fri, 24 May 2019 17:44:50 +0000",
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
	// Twilio RFC 2822 dates normalize to RFC3339, so the stored
	// checkpoint, the overlap window, and the wire format all see one
	// form.
	got, err = Normalize("Fri, 18 Oct 2019 17:02:00 +0000")
	if err != nil {
		t.Fatalf("Normalize = %v, want no error", err)
	}
	if want := "2019-10-18T17:02:00Z"; got != want {
		t.Errorf("Normalize = %q, want %q", got, want)
	}
	got, err = Normalize("Thu, 24 Aug 2023 05:01:45 +0000")
	if err != nil {
		t.Fatalf("Normalize = %v, want no error", err)
	}
	if want := "2023-08-24T05:01:45Z"; got != want {
		t.Errorf("Normalize = %q, want %q", got, want)
	}
}
