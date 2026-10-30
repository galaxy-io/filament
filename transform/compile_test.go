package transform

import (
	"slices"
	"strings"
	"testing"

	"github.com/galaxy-io/filament/rowmodel"
)

// TestRenameEntriesAreIndependent checks renames against each step's incoming schema.
// Dependent renames must be split into successive steps.
func TestRenameEntriesAreIndependent(t *testing.T) {
	tests := []struct {
		name    string
		steps   []Step
		want    []string
		wantErr string
	}{
		{
			name:    "source created by another entry",
			steps:   []Step{{Rename: map[string]string{"a": "b", "b": "c"}}},
			wantErr: "unknown column",
		},
		{
			name:    "destination freed by another entry",
			steps:   []Step{{Rename: map[string]string{"a": "b", "z": "a"}}},
			wantErr: `column "a" already exists`,
		},
		{
			name:    "destination not yet freed",
			steps:   []Step{{Rename: map[string]string{"a": "z", "z": "b"}}},
			wantErr: `column "z" already exists`,
		},
		{
			name:    "duplicate destination",
			steps:   []Step{{Rename: map[string]string{"a": "b", "z": "b"}}},
			wantErr: `column "b" already exists`,
		},
		{
			name:  "independent renames",
			steps: []Step{{Rename: map[string]string{"a": "b", "z": "c"}}},
			want:  []string{"b", "c"},
		},
		{
			name:  "dependent renames in separate steps",
			steps: []Step{{Rename: map[string]string{"a": "b"}}, {Rename: map[string]string{"b": "c", "z": "a"}}},
			want:  []string{"c", "a"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := &Definition{Version: 1, Resources: map[string]Resource{"items": {Steps: tt.steps}}}
			schema := rowmodel.Schema{
				Resource: "items",
				Fields: []rowmodel.Field{
					{Name: "a", Logical: rowmodel.LogicalString},
					{Name: "z", Logical: rowmodel.LogicalInt64},
				},
			}
			plan, err := Compile(def, schema)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Compile() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := schema.Clone()
			for i, name := range tt.want {
				want.Fields[i].Name = name
			}
			if got := plan.Schema(); !slices.Equal(got.Fields, want.Fields) {
				t.Fatalf("fields = %+v, want %+v", got.Fields, want.Fields)
			}
		})
	}
}
