package transform

import (
	_ "embed"
	"errors"
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
// outside the grammar, before any decoding into Definition. Violations come
// back as Errors, each issue at the path in the document it sits at.
func validateGrammar(yamlData []byte) error {
	sch, err := getGrammar()
	if err != nil {
		return err
	}
	var raw any
	if err := yaml.Unmarshal(yamlData, &raw); err != nil {
		return fmt.Errorf("parse yaml for grammar check: %w", err)
	}
	err = sch.Validate(normalizeForJSONSchema(raw))
	var ve *jsonschema.ValidationError
	if errors.As(err, &ve) {
		var errs Errors
		collectLeaves(ve, &errs)
		return errs.asError()
	}
	return err
}

// collectLeaves records the innermost causes of a schema violation, which are
// the ones that name a concrete problem rather than a failed alternative.
func collectLeaves(ve *jsonschema.ValidationError, errs *Errors) {
	if len(ve.Causes) == 0 {
		errs.addf(grammarPath(ve.InstanceLocation), "%s", ve.Message)
		return
	}
	for _, c := range ve.Causes {
		collectLeaves(c, errs)
	}
}

// grammarPath renders a JSON pointer in the same style the compiler uses for
// its issue paths, so a client maps both kinds the same way.
func grammarPath(pointer string) string {
	pointer = strings.TrimPrefix(pointer, "/")
	if pointer == "" {
		return "definition"
	}
	tokens := strings.Split(pointer, "/")
	var b strings.Builder
	for i, tok := range tokens {
		switch {
		case isIndex(tok):
			fmt.Fprintf(&b, "[%s]", tok)
		case i > 0 && (tokens[i-1] == "resources" || tokens[i-1] == "compute" || tokens[i-1] == "rename"):
			fmt.Fprintf(&b, "[%q]", tok)
		case i == 0:
			b.WriteString(tok)
		default:
			b.WriteString("." + tok)
		}
	}
	return b.String()
}

func isIndex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
