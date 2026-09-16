package transform

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

// grammar.v1.json is the JSON schema every definition must satisfy: the
// step forms, the expression forms, and the set of function names. It checks
// shape only; types are Compile's job.
//
//go:embed grammar.v1.json
var grammarJSON []byte

var (
	compiledOnce sync.Once
	compiled     *jsonschema.Schema
	compileErr   error
)

// getGrammar compiles the embedded schema once and caches it.
func getGrammar() (*jsonschema.Schema, error) {
	compiledOnce.Do(func() {
		c := jsonschema.NewCompiler()
		if err := c.AddResource("transform.v1.json", strings.NewReader(string(grammarJSON))); err != nil {
			compileErr = fmt.Errorf("add grammar resource: %w", err)
			return
		}
		compiled, compileErr = c.Compile("transform.v1.json")
	})
	return compiled, compileErr
}

// validateGrammar rejects a document whose shape or function names fall
// outside the grammar, before any decoding into Definition.
func validateGrammar(yamlData []byte) error {
	sch, err := getGrammar()
	if err != nil {
		return err
	}
	var raw any
	if err := yaml.Unmarshal(yamlData, &raw); err != nil {
		return fmt.Errorf("parse yaml for grammar check: %w", err)
	}
	return sch.Validate(normalizeForJSONSchema(raw))
}

// normalizeForJSONSchema rewrites the map[any]any values yaml.v3 produces
// into map[string]any, which is what the schema validator walks.
func normalizeForJSONSchema(v any) any {
	switch x := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[fmt.Sprintf("%v", k)] = normalizeForJSONSchema(val)
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
