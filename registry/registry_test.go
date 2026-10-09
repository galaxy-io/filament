package registry

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
)

type specSource struct {
	filament.Source
	name string
}

func (s *specSource) Spec() filament.ConnectorSpec { return filament.ConnectorSpec{Name: s.name} }

func TestSourceAliases(t *testing.T) {
	sources := NewSources()
	sources.RegisterWithMaturity("example@v1", filament.MaturityBeta, func() filament.Source { return &specSource{name: "example@v1"} })
	sources.RegisterAlias("example", "example@v1")
	first, err := sources.Resolve("example")
	if err != nil {
		t.Fatal(err)
	}
	second, err := sources.Resolve("example@v1")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || first.Spec().Name != "example@v1" {
		t.Fatal("alias must resolve fresh instances with concrete identity")
	}
	spec, err := sources.Spec("example")
	if err != nil || spec.Maturity != filament.MaturityBeta {
		t.Fatalf("alias spec: %+v, %v", spec, err)
	}
	if specs := sources.Specs(); len(specs) != 2 || specs[0].Name != "example" || specs[1].Name != "example@v1" || specs[0].AliasTarget != "example@v1" || specs[1].AliasTarget != "" {
		t.Fatalf("specs=%+v", specs)
	}
	if _, err := sources.Resolve("unknown"); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("unknown source error=%v", err)
	}
	for _, test := range []struct {
		name   string
		action func()
	}{
		{"duplicate alias", func() { sources.RegisterAlias("example", "example@v1") }},
		{"alias over concrete", func() { sources.RegisterAlias("example@v1", "example@v1") }},
		{"concrete over alias", func() { sources.Register("example", func() filament.Source { return &specSource{} }) }},
		{"missing target", func() { sources.RegisterAlias("missing", "unknown") }},
		{"alias chain", func() { sources.RegisterAlias("chain", "example") }},
		{"empty alias", func() { sources.RegisterAlias("", "example@v1") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected registration panic")
				}
			}()
			test.action()
		})
	}
}

func TestSourcesWithOverrides(t *testing.T) {
	base := NewSources()
	for _, name := range []string{"example@v1", "other"} {
		base.RegisterWithMaturity(name, filament.MaturityBeta, func() filament.Source { return &specSource{name: name} })
	}
	base.RegisterAlias("example", "example@v1")
	private := NewSources()
	private.RegisterWithMaturity("example@v2", filament.MaturityAlpha, func() filament.Source { return &specSource{name: "example@v2"} })
	private.Register("private@v3", func() filament.Source { return &specSource{name: "private@v3"} })
	private.RegisterAlias("example", "example@v2")
	private.RegisterAlias("private", "private@v3")
	merged, err := base.WithOverrides(private)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"other", "example@v1", "private", "private@v3"} {
		if _, err := merged.Resolve(key); err != nil {
			t.Fatal(err)
		}
	}
	spec, err := merged.Spec("example")
	if err != nil || spec.Name != "example@v2" || spec.Maturity != filament.MaturityAlpha {
		t.Fatalf("override spec=%+v err=%v", spec, err)
	}
	original, err := base.Spec("example")
	if err != nil || original.Name != "example@v1" {
		t.Fatalf("base alias mutated: %+v, %v", original, err)
	}
	if _, err := base.Resolve("example@v2"); !errors.Is(err, ErrUnknownProvider) {
		t.Fatal("private version leaked into base")
	}
	if _, err := base.Resolve("private"); !errors.Is(err, ErrUnknownProvider) {
		t.Fatal("private source leaked into base")
	}
	merged.Register("only-merged", func() filament.Source { return &specSource{name: "only-merged"} })
	if _, err := private.Resolve("only-merged"); !errors.Is(err, ErrUnknownProvider) {
		t.Fatal("merged source leaked into overrides")
	}
}

func TestSourcesWithOverridesRejectsInvalidInputs(t *testing.T) {
	base := NewSources()
	base.Register("target", func() filament.Source { return &specSource{name: "target"} })
	base.RegisterAlias("alias", "target")
	overrides := NewSources()
	overrides.Register("replacement", func() filament.Source { return &specSource{name: "replacement"} })
	overrides.RegisterAlias("target", "replacement")
	if merged, err := base.WithOverrides(overrides); err == nil || merged != nil {
		t.Fatal("alias replaced an existing concrete target")
	}
	if merged, err := base.WithOverrides(nil); err == nil || merged != nil {
		t.Fatal("nil overrides accepted")
	}
}

func TestSourcesWithOverridesRejectsRegistrationReplacement(t *testing.T) {
	for _, name := range []string{"example", "example@v1"} {
		for _, replacementAlias := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/alias=%t", name, replacementAlias), func(t *testing.T) {
				base := NewSources()
				base.RegisterWithMaturity(name, filament.MaturityStable, func() filament.Source { return &specSource{name: name} })
				private := NewSources()
				if replacementAlias {
					private.Register("replacement", func() filament.Source { return &specSource{name: "replacement"} })
					private.RegisterAlias(name, "replacement")
				} else {
					private.Register(name, func() filament.Source { return &specSource{name: name} })
				}
				if merged, err := base.WithOverrides(private); err == nil || merged != nil || !strings.Contains(err.Error(), name) {
					t.Fatalf("replacement accepted: %v, %v", merged, err)
				}
				spec, err := base.Spec(name)
				if err != nil || spec.Name != name || spec.Maturity != filament.MaturityStable {
					t.Fatalf("original registration changed: %+v, %v", spec, err)
				}
			})
		}
	}
}

func TestSourcesWithOverridesRejectsConcreteOverAlias(t *testing.T) {
	base := NewSources()
	base.Register("example@v1", func() filament.Source { return &specSource{name: "example@v1"} })
	base.RegisterAlias("example", "example@v1")
	private := NewSources()
	private.Register("example", func() filament.Source { return &specSource{name: "example"} })
	if merged, err := base.WithOverrides(private); err == nil || merged != nil {
		t.Fatalf("concrete replacement of alias accepted: %v, %v", merged, err)
	}
}

type streamSpecSource struct{ filament.Source }

func (streamSpecSource) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{Name: "stream", Stream: &filament.StreamCapabilities{}}
}

type streamSource struct {
	streamSpecSource
	filament.StreamSource
	filament.ReplicationStreamPlanner
}

type streamSpecSink struct{ filament.Sink }

func (streamSpecSink) Spec() filament.SinkSpec {
	return filament.SinkSpec{Name: "stream", Capabilities: filament.SinkCapabilities{Stream: &filament.StreamingSinkCapabilities{}}}
}

type streamSink struct {
	streamSpecSink
	filament.StreamingSink
}

func TestRegisterRequiresDeclaredContracts(t *testing.T) {
	mustPanic := func(t *testing.T, want string, register func()) {
		t.Helper()
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected registration panic")
			}
			if msg := fmt.Sprint(r); !strings.Contains(msg, want) {
				t.Fatalf("panic %q does not mention %q", msg, want)
			}
		}()
		register()
	}
	t.Run("source without stream contracts", func(t *testing.T) {
		mustPanic(t, "filament.StreamSource", func() {
			NewSources().Register("stream", func() filament.Source { return streamSpecSource{} })
		})
	})
	t.Run("source with stream contracts", func(t *testing.T) {
		sources := NewSources()
		sources.Register("stream", func() filament.Source { return streamSource{} })
		if _, err := sources.Spec("stream"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("sink without stream contract", func(t *testing.T) {
		mustPanic(t, "filament.StreamingSink", func() {
			NewSinks().Register("stream", func() filament.Sink { return streamSpecSink{} })
		})
	})
	t.Run("sink with stream contract", func(t *testing.T) {
		sinks := NewSinks()
		sinks.Register("stream", func() filament.Sink { return streamSink{} })
		if _, err := sinks.Spec("stream"); err != nil {
			t.Fatal(err)
		}
	})
}
