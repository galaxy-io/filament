package transform

import (
	"errors"
	"testing"
)

// A definition that fails the grammar reports one issue per real problem,
// not one per schema alternative that did not match.
func TestValidateGrammarCollapsesAlternatives(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		path string
	}{
		{
			name: "step with no operation",
			doc:  `{"version":1,"resources":{"r":{"steps":[{"where":{"col":"a"}}]}}}`,
			path: `resources["r"].steps[0]`,
		},
		{
			name: "unknown function",
			doc:  `{"version":1,"resources":{"r":{"steps":[{"compute":{"x":{"nope":{"col":"a"}}}}]}}}`,
			path: `resources["r"].steps[0].compute["x"].nope`,
		},
		{
			name: "empty argument list",
			doc:  `{"version":1,"resources":{"r":{"steps":[{"compute":{"x":{"trim":[]}}}]}}}`,
			path: `resources["r"].steps[0].compute["x"].trim`,
		},
		{
			name: "empty compute name",
			doc:  `{"version":1,"resources":{"r":{"steps":[{"compute":{"":"hello"}}]}}}`,
			path: `resources["r"].steps[0].compute`,
		},
		{
			name: "empty rename source",
			doc:  `{"version":1,"resources":{"r":{"steps":[{"rename":{"":"b"}}]}}}`,
			path: `resources["r"].steps[0].rename`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGrammar([]byte(tc.doc))
			var errs *Errors
			if !errors.As(err, &errs) {
				t.Fatalf("validateGrammar() = %v, want *Errors", err)
			}
			if len(errs.Issues) != 1 {
				t.Fatalf("got %d issues, want 1: %v", len(errs.Issues), errs.Issues)
			}
			if got := errs.Issues[0].Path; got != tc.path {
				t.Errorf("issue path = %q, want %q", got, tc.path)
			}
		})
	}
}

func TestValidateGrammarAcceptsEveryStepShape(t *testing.T) {
	doc := `{"version":1,"resources":{"r":{"steps":[
		{"rename":{"a":"b"}},
		{"drop":["c"]},
		{"compute":{"x":{"concat":[{"col":"a"},"!"]}},"where":{"and":[{"is_null":{"col":"a"}},{"gt":[{"col":"n"},1]}]}}
	]}}}`
	if err := validateGrammar([]byte(doc)); err != nil {
		t.Fatalf("validateGrammar() = %v", err)
	}
}
