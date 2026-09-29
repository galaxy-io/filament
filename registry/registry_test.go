package registry

import (
	"errors"
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
	for _, name := range []string{"example@v1", "example@v2", "other"} {
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
	original, err = base.Spec("example@v2")
	if err != nil || original.Maturity != filament.MaturityBeta {
		t.Fatalf("base version mutated: %+v, %v", original, err)
	}
	if _, err := base.Resolve("private"); !errors.Is(err, ErrUnknownProvider) {
		t.Fatal("private source leaked into base")
	}
	merged.Register("only-merged", func() filament.Source { return &specSource{name: "only-merged"} })
	if _, err := private.Resolve("only-merged"); !errors.Is(err, ErrUnknownProvider) {
		t.Fatal("merged source leaked into overrides")
	}
}

func TestSourcesWithOverridesRejectsBrokenAliases(t *testing.T) {
	base := NewSources()
	base.Register("target", func() filament.Source { return &specSource{name: "target"} })
	base.RegisterAlias("alias", "target")
	overrides := NewSources()
	overrides.Register("replacement", func() filament.Source { return &specSource{name: "replacement"} })
	overrides.RegisterAlias("target", "replacement")
	if merged, err := base.WithOverrides(overrides); err == nil || merged != nil {
		t.Fatal("overrides introduced an alias chain")
	}
	if merged, err := base.WithOverrides(nil); err == nil || merged != nil {
		t.Fatal("nil overrides accepted")
	}
}
