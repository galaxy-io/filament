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
	if specs := sources.Specs(); len(specs) != 2 || specs[0].Name != "example" || specs[1].Name != "example@v1" {
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
