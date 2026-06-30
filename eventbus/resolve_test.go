package eventbus

import (
	"errors"
	"testing"
)

func TestLiteralPrefix(t *testing.T) {
	cases := []struct {
		pattern string
		prefix  string
		exact   bool
	}{
		{"some.message.*.type.>", "some.message", false},
		{"some.message.type.*.>", "some.message.type", false},
		{"a.b.c", "a.b.c", true}, // no wildcards → fully literal
		{"*.foo", "", false},     // leading wildcard → empty prefix
		{">", "", false},
		{"some.>", "some", false},
	}
	for _, c := range cases {
		prefix, exact := LiteralPrefix(c.pattern)
		if prefix != c.prefix || exact != c.exact {
			t.Errorf("LiteralPrefix(%q) = (%q, %v), want (%q, %v)", c.pattern, prefix, exact, c.prefix, c.exact)
		}
	}
}

// Passthrough (NATS): Target == pattern, broker matches exactly.
func TestPassthroughResolver(t *testing.T) {
	r, err := PassthroughResolver{}.Resolve("some.message.*.type.>")
	if err != nil {
		t.Fatal(err)
	}
	if r.Target != "some.message.*.type.>" {
		t.Errorf("Target = %q", r.Target)
	}
	if !r.Exact {
		t.Error("Exact = false, want true (broker expresses */>)")
	}
}

// PrefixTopic (Kafka-esc style): Target is the leading literal run, ClientFilter the rest.
func TestPrefixTopicResolver(t *testing.T) {
	r, err := PrefixTopicResolver{}.Resolve("some.message.*.type.>")
	if err != nil {
		t.Fatal(err)
	}
	if r.Target != "some.message" {
		t.Errorf("Target = %q, want some.message", r.Target)
	}
	if r.Exact {
		t.Error("Exact = true, want false (topic is a superset)")
	}
	// ClientFilter must do exactly what the full pattern means.
	keep := r.ClientFilter.MatchSubject("some.message.A.type.x.y")
	drop := r.ClientFilter.MatchSubject("some.message.A.other.z")
	if !keep || drop {
		t.Errorf("ClientFilter: keep=%v drop=%v, want true,false", keep, drop)
	}

	// A fully-literal pattern is exact — the topic is the whole subject.
	lit, _ := PrefixTopicResolver{}.Resolve("some.message.type")
	if lit.Target != "some.message.type" || !lit.Exact {
		t.Errorf("literal resolve = (%q, exact=%v)", lit.Target, lit.Exact)
	}
}

// Every resolver propagates a bad pattern through the shared grammar parse.
func TestResolversRejectBadPattern(t *testing.T) {
	for _, r := range []Resolver{PassthroughResolver{}, PrefixTopicResolver{}} {
		if _, err := r.Resolve("a.>.b"); !errors.Is(err, ErrBadPattern) {
			t.Errorf("%T.Resolve(bad) = %v, want ErrBadPattern", r, err)
		}
	}
}
