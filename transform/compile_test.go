package transform

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/galaxy-io/filament/rowmodel"
)

// chatMessageSchema mirrors the gxdb chat_message resource the builder is
// exercised against, so these paths match what the UI sees.
func chatMessageSchema() rowmodel.Schema {
	return rowmodel.Schema{
		Resource:   "chat_message",
		PrimaryKey: []string{"id"},
		Fields: []rowmodel.Field{
			{Name: "id", Logical: rowmodel.LogicalUUID},
			{Name: "created_at", Logical: rowmodel.LogicalTimestampTZ},
			{Name: "tenant_id", Logical: rowmodel.LogicalUUID},
			{Name: "project_id", Logical: rowmodel.LogicalUUID},
			{Name: "chat_session_id", Logical: rowmodel.LogicalUUID},
			{Name: "type", Logical: rowmodel.LogicalString},
			{Name: "model", Logical: rowmodel.LogicalString},
			{Name: "created_by_user_id", Logical: rowmodel.LogicalUUID},
			{Name: "content", Logical: rowmodel.LogicalString},
			{Name: "status", Logical: rowmodel.LogicalString},
			{Name: "tool_calls", Logical: rowmodel.LogicalJSON},
			{Name: "input_tokens", Logical: rowmodel.LogicalInt32},
			{Name: "output_tokens", Logical: rowmodel.LogicalInt32},
			{Name: "total_tokens", Logical: rowmodel.LogicalInt32},
			{Name: "duration", Logical: rowmodel.LogicalInt32},
		},
	}
}

// analyzeStep parses a one-step chat_message definition and analyzes it.
func analyzeStep(t *testing.T, step string) ([]ExpressionType, error) {
	t.Helper()
	doc := fmt.Sprintf(`{"version":1,"resources":{"chat_message":{"steps":[%s]}}}`, step)
	def, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse(%s) = %v", step, err)
	}
	_, analysis, err := Analyze(def, chatMessageSchema())
	return analysis.Types, err
}

func typeMap(types []ExpressionType) map[string]string {
	out := make(map[string]string, len(types))
	for _, et := range types {
		out[et.Path] = string(et.Logical)
	}
	return out
}

// The path of every sub-expression is part of the API a builder is written
// against: one entry per fixture the UI regression walk uses, pinned.
func TestAnalyzeExpressionTypePaths(t *testing.T) {
	const p = `resources["chat_message"].steps[0]`
	cases := []struct {
		name string
		step string
		want map[string]string
	}{
		{
			name: "six-function chain",
			step: `{"compute":{"content_preview":{"length":{"concat":[{"substring":[{"replace":[{"lower":{"trim":{"col":"content"}}},"a","b"]},1,10]},"!"]}}}}`,
			want: map[string]string{
				p + `.compute["content_preview"]`:                                                              "int64",
				p + `.compute["content_preview"].length[0]`:                                                    "string",
				p + `.compute["content_preview"].length[0].concat[0]`:                                          "string",
				p + `.compute["content_preview"].length[0].concat[1]`:                                          "string",
				p + `.compute["content_preview"].length[0].concat[0].substring[0]`:                             "string",
				p + `.compute["content_preview"].length[0].concat[0].substring[1]`:                             "int64",
				p + `.compute["content_preview"].length[0].concat[0].substring[2]`:                             "int64",
				p + `.compute["content_preview"].length[0].concat[0].substring[0].replace[0]`:                  "string",
				p + `.compute["content_preview"].length[0].concat[0].substring[0].replace[1]`:                  "string",
				p + `.compute["content_preview"].length[0].concat[0].substring[0].replace[2]`:                  "string",
				p + `.compute["content_preview"].length[0].concat[0].substring[0].replace[0].lower[0]`:         "string",
				p + `.compute["content_preview"].length[0].concat[0].substring[0].replace[0].lower[0].trim[0]`: "string",
			},
		},
		{
			name: "function nested in argument 2",
			step: `{"compute":{"tokens":{"add":[{"col":"output_tokens"},{"abs":{"col":"total_tokens"}}]}}}`,
			want: map[string]string{
				p + `.compute["tokens"]`:               "int32",
				p + `.compute["tokens"].add[0]`:        "int32",
				p + `.compute["tokens"].add[1]`:        "int32",
				p + `.compute["tokens"].add[1].abs[0]`: "int32",
			},
		},
		{
			name: "compact where with a widened literal",
			step: `{"compute":{"status":"sent"},"where":{"and":[{"is_null":{"col":"created_by_user_id"}},{"gt":[{"col":"output_tokens"},100]}]}}`,
			want: map[string]string{
				p + `.compute["status"]`:       "string",
				p + `.where`:                   "bool",
				p + `.where.and[0]`:            "bool",
				p + `.where.and[0].is_null[0]`: "uuid",
				p + `.where.and[1]`:            "bool",
				p + `.where.and[1].gt[0]`:      "int32",
				p + `.where.and[1].gt[1]`:      "int32",
			},
		},
		{
			name: "where that is not a compact group",
			step: `{"compute":{"content":{"coalesce":[{"col":"content"},"(empty)"]}},"where":{"not":{"is_null":{"col":"created_by_user_id"}}}}`,
			want: map[string]string{
				p + `.compute["content"]`:             "string",
				p + `.compute["content"].coalesce[0]`: "string",
				p + `.compute["content"].coalesce[1]`: "string",
				p + `.where`:                          "bool",
				p + `.where.not[0]`:                   "bool",
				p + `.where.not[0].is_null[0]`:        "uuid",
			},
		},
		{name: "multi-column rename", step: `{"rename":{"type":"kind","model":"llm"}}`, want: map[string]string{}},
		{name: "multi-column drop", step: `{"drop":["duration","input_tokens"]}`, want: map[string]string{}},
		{name: "literal root", step: `{"compute":{"src":"gxdb"}}`, want: map[string]string{p + `.compute["src"]`: "string"}},
		{
			name: "multi-output",
			step: `{"compute":{"a":{"upper":{"col":"type"}},"b":{"lower":{"col":"model"}}}}`,
			want: map[string]string{
				p + `.compute["a"]`:          "string",
				p + `.compute["a"].upper[0]`: "string",
				p + `.compute["b"]`:          "string",
				p + `.compute["b"].lower[0]`: "string",
			},
		},
		{
			name: "literal-first concat",
			step: `{"compute":{"content":{"concat":["prefix:",{"col":"content"}]}}}`,
			want: map[string]string{
				p + `.compute["content"]`:           "string",
				p + `.compute["content"].concat[0]`: "string",
				p + `.compute["content"].concat[1]`: "string",
			},
		},
		{
			name: "uuid coalesce",
			step: `{"compute":{"tenant_id":{"coalesce":[{"col":"tenant_id"},{"col":"project_id"}]}}}`,
			want: map[string]string{
				p + `.compute["tenant_id"]`:             "uuid",
				p + `.compute["tenant_id"].coalesce[0]`: "uuid",
				p + `.compute["tenant_id"].coalesce[1]`: "uuid",
			},
		},
		{
			name: "bool chain",
			step: `{"compute":{"has_user":{"not":{"is_null":{"col":"created_by_user_id"}}}}}`,
			want: map[string]string{
				p + `.compute["has_user"]`:                   "bool",
				p + `.compute["has_user"].not[0]`:            "bool",
				p + `.compute["has_user"].not[0].is_null[0]`: "uuid",
			},
		},
		{
			name: "date chain",
			step: `{"compute":{"yr":{"year":{"to_date":{"col":"model"}}}}}`,
			want: map[string]string{
				p + `.compute["yr"]`:                    "int64",
				p + `.compute["yr"].year[0]`:            "date",
				p + `.compute["yr"].year[0].to_date[0]`: "string",
			},
		},
		{
			name: "nesting in argument 1",
			step: `{"compute":{"label":{"concat":[{"upper":{"col":"status"}},{"col":"type"}]}}}`,
			want: map[string]string{
				p + `.compute["label"]`:                    "string",
				p + `.compute["label"].concat[0]`:          "string",
				p + `.compute["label"].concat[0].upper[0]`: "string",
				p + `.compute["label"].concat[1]`:          "string",
			},
		},
		{
			name: "in-place with type change",
			step: `{"compute":{"content":{"length":{"col":"content"}}}}`,
			want: map[string]string{
				p + `.compute["content"]`:           "int64",
				p + `.compute["content"].length[0]`: "string",
			},
		},
		{
			name: "three-deep condition groups",
			step: `{"compute":{"status":"archived"},"where":{"and":[{"is_null":{"col":"created_by_user_id"}},{"or":[{"eq":[{"col":"status"},"sent"]},{"and":[{"eq":[{"col":"status"},"done"]},{"gt":[{"col":"output_tokens"},100]}]}]}]}}`,
			want: map[string]string{
				p + `.compute["status"]`:               "string",
				p + `.where`:                           "bool",
				p + `.where.and[0]`:                    "bool",
				p + `.where.and[0].is_null[0]`:         "uuid",
				p + `.where.and[1]`:                    "bool",
				p + `.where.and[1].or[0]`:              "bool",
				p + `.where.and[1].or[0].eq[0]`:        "string",
				p + `.where.and[1].or[0].eq[1]`:        "string",
				p + `.where.and[1].or[1]`:              "bool",
				p + `.where.and[1].or[1].and[0]`:       "bool",
				p + `.where.and[1].or[1].and[0].eq[0]`: "string",
				p + `.where.and[1].or[1].and[0].eq[1]`: "string",
				p + `.where.and[1].or[1].and[1]`:       "bool",
				p + `.where.and[1].or[1].and[1].gt[0]`: "int32",
				p + `.where.and[1].or[1].and[1].gt[1]`: "int32",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			types, err := analyzeStep(t, tc.step)
			if err != nil {
				t.Fatalf("Analyze() = %v", err)
			}
			got := typeMap(types)
			if len(got) != len(types) {
				t.Errorf("Analyze() reported a path twice: %v", types)
			}
			for path, want := range tc.want {
				if got[path] != want {
					t.Errorf("type at %s = %q, want %q", path, got[path], want)
				}
			}
			for path, typ := range got {
				if _, ok := tc.want[path]; !ok {
					t.Errorf("unexpected type at %s = %q", path, typ)
				}
			}
		})
	}
}

// A compile failure names the argument responsible when one is, and still
// reports the types of the parts that compiled.
func TestAnalyzeReportsArgumentPathsAndPartialTypes(t *testing.T) {
	const p = `resources["chat_message"].steps[0]`
	cases := []struct {
		name    string
		step    string
		path    string
		message string
		typed   []string
		untyped []string
	}{
		{
			name:    "drop primary key",
			step:    `{"drop":["id"]}`,
			path:    p + `.drop[0]`,
			message: "cannot drop primary key",
		},
		{
			name:    "rename onto an existing column",
			step:    `{"rename":{"id":"created_at"}}`,
			path:    p + `.rename["id"]`,
			message: "already exists",
		},
		{
			name:    "input type mismatch",
			step:    `{"compute":{"x":{"trim":{"col":"input_tokens"}}}}`,
			path:    p + `.compute["x"].trim[0]`,
			message: "trim expects string for value, got int32",
			typed:   []string{p + `.compute["x"].trim[0]`},
			untyped: []string{p + `.compute["x"]`},
		},
		{
			name:    "pattern does not compile",
			step:    `{"compute":{"x":{"regex_match":[{"col":"content"},"("]}}}`,
			path:    p + `.compute["x"].regex_match[1]`,
			message: "regex_match pattern",
			typed:   []string{p + `.compute["x"].regex_match[0]`, p + `.compute["x"].regex_match[1]`},
			untyped: []string{p + `.compute["x"]`},
		},
		{
			name:    "substring start below 1",
			step:    `{"compute":{"x":{"substring":[{"col":"content"},0]}}}`,
			path:    p + `.compute["x"].substring[1]`,
			message: "must be 1 or more",
		},
		{
			name:    "mismatched operand in a nested argument",
			step:    `{"compute":{"x":{"add":[{"col":"output_tokens"},{"length":{"col":"content"}}]}}}`,
			path:    p + `.compute["x"].add[1]`,
			message: "matching types",
			typed:   []string{p + `.compute["x"].add[0]`, p + `.compute["x"].add[1]`, p + `.compute["x"].add[1].length[0]`},
			untyped: []string{p + `.compute["x"]`},
		},
		{
			name:    "where cannot change type",
			step:    `{"compute":{"content":{"length":{"col":"content"}}},"where":{"eq":[{"col":"status"},"sent"]}}`,
			path:    p + `.compute["content"]`,
			message: "where cannot change type",
			typed:   []string{p + `.compute["content"]`, p + `.where`},
		},
		{
			name:    "where must be bool",
			step:    `{"compute":{"status":{"col":"status"}},"where":{"col":"content"}}`,
			path:    p + `.where`,
			message: "expects bool, got string",
			typed:   []string{p + `.where`},
		},
		{
			name:    "wrong arity stays on the call",
			step:    `{"compute":{"x":{"replace":[{"col":"content"},"a"]}}}`,
			path:    p + `.compute["x"]`,
			message: "replace expects 3 arguments, got 2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			types, err := analyzeStep(t, tc.step)
			var errs *Errors
			if !errors.As(err, &errs) {
				t.Fatalf("Analyze() = %v, want *Errors", err)
			}
			if len(errs.Issues) != 1 {
				t.Fatalf("got %d issues, want 1: %v", len(errs.Issues), errs.Issues)
			}
			if got := errs.Issues[0]; got.Path != tc.path || !strings.Contains(got.Message, tc.message) {
				t.Errorf("issue = %s: %s, want %s: …%s…", got.Path, got.Message, tc.path, tc.message)
			}
			got := typeMap(types)
			for _, path := range tc.typed {
				if _, ok := got[path]; !ok {
					t.Errorf("no type reported at %s; got %v", path, got)
				}
			}
			for _, path := range tc.untyped {
				if typ, ok := got[path]; ok {
					t.Errorf("type reported at failed path %s = %q", path, typ)
				}
			}
		})
	}
}

func TestParseRejectsUnknownFunction(t *testing.T) {
	doc := `{"version":1,"resources":{"chat_message":{"steps":[{"compute":{"x":{"nope":{"col":"content"}}}}]}}}`
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("Parse accepted an unknown function")
	}
}
