package embedding_test

import (
	"testing"

	"github.com/galaxy-io/filament/connectors/internal/embedding"
)

func TestDeterministicUUID(t *testing.T) {
	a := embedding.DeterministicUUID("user-abc")
	if a != embedding.DeterministicUUID("user-abc") {
		t.Fatal("same key produced different UUIDs")
	}
	if a.Version() != 5 {
		t.Fatalf("version = %d, want 5", a.Version())
	}
	if embedding.DeterministicUUID("001") == embedding.DeterministicUUID("1") {
		t.Fatal(`"001" and "1" produced the same UUID`)
	}
}

func TestCanonicalUint64(t *testing.T) {
	tests := []struct {
		key  string
		want uint64
		ok   bool
	}{
		{"0", 0, true},
		{"42", 42, true},
		{"18446744073709551615", 18446744073709551615, true},
		{"042", 0, false},
		{"+42", 0, false},
		{" 42", 0, false},
		{"-1", 0, false},
		{"18446744073709551616", 0, false},
		{"user-abc", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		got, ok := embedding.CanonicalUint64(tt.key)
		if got != tt.want || ok != tt.ok {
			t.Errorf("CanonicalUint64(%q) = %d, %v; want %d, %v", tt.key, got, ok, tt.want, tt.ok)
		}
	}
}

func TestCanonicalUUID(t *testing.T) {
	const canonical = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	if got, ok := embedding.CanonicalUUID(canonical); !ok || got.String() != canonical {
		t.Fatalf("CanonicalUUID(%q) = %v, %v; want it accepted", canonical, got, ok)
	}
	for _, key := range []string{
		"6BA7B810-9DAD-11D1-80B4-00C04FD430C8",
		"{6ba7b810-9dad-11d1-80b4-00c04fd430c8}",
		"6ba7b8109dad11d180b400c04fd430c8",
		"urn:uuid:6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"not-a-uuid",
	} {
		if _, ok := embedding.CanonicalUUID(key); ok {
			t.Errorf("CanonicalUUID(%q) accepted a non-canonical spelling", key)
		}
	}
}
