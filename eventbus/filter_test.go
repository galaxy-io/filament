package eventbus

import (
	"errors"
	"testing"
)

// Compile + Match must agree with the reference SubjectMatch on every case.
func TestFilterEquivSubjectMatch(t *testing.T) {
	patterns := []string{
		"app.v1.run.t1.r1.started",
		"app.v1.*.t1.r1.*",
		"app.v1.*.t1.*.*",
		"app.v1.batch.*.*.written",
		"app.v1.>",
		"app.>",
		"*.v1.run.t1.r1.started",
	}
	subjects := []string{
		"app.v1.run.t1.r1.started",
		"app.v1.run.t1.r1.completed",
		"app.v1.batch.t1.r1.written",
		"app.v1.batch.t2.r9.written",
		"app.v1.run.t1.r9.started",
		"other.v1.run.t1.r1.started",
		"app.v1.run.t1.r1.started.extra",
	}
	for _, p := range patterns {
		f, err := Compile(p)
		if err != nil {
			t.Fatalf("Compile(%q): %v", p, err)
		}
		for _, s := range subjects {
			if got, want := f.Match(split(s)), SubjectMatch(p, s); got != want {
				t.Errorf("pattern %q subject %q: Filter=%v SubjectMatch=%v", p, s, got, want)
			}
		}
	}
}

func TestCompileErrors(t *testing.T) {
	for _, p := range []string{"", "app.>.v1", "a.>.b.c"} {
		if _, err := Compile(p); !errors.Is(err, ErrBadPattern) {
			t.Errorf("Compile(%q) = %v, want ErrBadPattern", p, err)
		}
	}
}

func TestValidToken(t *testing.T) {
	for _, ok := range []string{"t1", "run", "page_fetched"} {
		if !IsValidToken(ok) {
			t.Errorf("IsValidToken(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "a.b", "a*", "a>", "a b"} {
		if IsValidToken(bad) {
			t.Errorf("IsValidToken(%q) = true, want false", bad)
		}
	}
}

func split(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == '.' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	return append(out, cur)
}
