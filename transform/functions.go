package transform

import (
	"maps"
	"slices"

	"github.com/galaxy-io/filament/rowmodel"
)

// GrammarVersion is the version a definition must declare.
func GrammarVersion() int { return supportedVersion }

// Grammar is the JSON schema every definition must satisfy, for clients that
// want to check a document's shape before sending it.
func Grammar() []byte { return slices.Clone(grammarJSON) }

// Functions returns every catalog function's signature, sorted by name.
func Functions() []FunctionSpec {
	specs := make([]FunctionSpec, 0, len(catalog))
	for _, name := range slices.Sorted(maps.Keys(catalog)) {
		specs = append(specs, catalog[name].spec)
	}
	return specs
}

// FunctionsFor returns the functions that accept a column of the given type
// in some argument, sorted by name. It is the same acceptance the compiler
// applies, so a builder offering these never proposes a call Compile rejects.
func FunctionsFor(t rowmodel.LogicalType) []FunctionSpec {
	var out []FunctionSpec
	for _, spec := range Functions() {
		for _, a := range spec.Args {
			if !a.Literal && (len(a.Types) == 0 || slices.Contains(a.Types, t)) {
				out = append(out, spec)
				break
			}
		}
	}
	return out
}
