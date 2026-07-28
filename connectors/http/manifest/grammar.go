// grammar.go enforces the v1 manifest meta-grammar — the shape every
// manifest.yaml must conform to (object structure, field types, enum values).
//
// Two-layer validation keeps responsibilities separate:
//
//   - Grammar (this file)        — shape: required objects, field types,
//     enum membership. Authoritative source is grammar.v1.json; the matching
//     enum constants in grammar_enums.go are mirrored for use by Go-side
//     semantics validation.
//   - Semantics (validate.go)    — cross-field rules: parent cycles,
//     checkpoint-key collisions, template syntax against scope subsets.
//
// ValidateGrammar runs first inside Parse; on success the YAML is decoded
// into the typed Manifest and validateSemantics finishes the load-time gate.
package manifest

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

//go:embed grammar.v1.json
var grammarJSON []byte

var (
	compiledOnce sync.Once
	compiled     *jsonschema.Schema
	compileErr   error
)

func getGrammar() (*jsonschema.Schema, error) {
	compiledOnce.Do(func() {
		c := jsonschema.NewCompiler()
		if err := c.AddResource("manifest.v1.json", strings.NewReader(string(grammarJSON))); err != nil {
			compileErr = fmt.Errorf("add grammar resource: %w", err)
			return
		}
		compiled, compileErr = c.Compile("manifest.v1.json")
	})
	return compiled, compileErr
}

// ValidateGrammar checks that yamlData conforms to the v1 manifest meta-grammar
// (shape, field types, enum membership). Cross-field semantics live in
// validateSemantics — the two run sequentially inside Parse.
func ValidateGrammar(yamlData []byte) error {
	sch, err := getGrammar()
	if err != nil {
		return err
	}
	var raw any
	if err := yaml.Unmarshal(yamlData, &raw); err != nil {
		return fmt.Errorf("parse yaml for grammar check: %w", err)
	}
	raw = normalizeForJSONSchema(raw)
	if err := sch.Validate(raw); err != nil {
		return err
	}
	return nil
}

// normalizeForJSONSchema converts yaml.v3's map[any]any into map[string]any
// so santhosh-tekuri/jsonschema can walk the tree.
func normalizeForJSONSchema(v any) any {
	switch x := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			ks, ok := k.(string)
			if !ok {
				ks = fmt.Sprintf("%v", k)
			}
			out[ks] = normalizeForJSONSchema(val)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = normalizeForJSONSchema(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = normalizeForJSONSchema(val)
		}
		return out
	default:
		return v
	}
}
