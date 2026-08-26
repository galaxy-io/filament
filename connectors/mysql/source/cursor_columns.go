package mysql

import (
	"context"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

var cursorNamePriority = []string{
	"updated_at", "modified_at", "last_modified_at", "last_updated_at", "updated", "modified",
}

// CursorColumns reports every table column and recommends conventional
// timestamp fields that can act as durable incremental watermarks.
func (s *Source) CursorColumns(ctx context.Context, table string) ([]filament.CursorColumn, error) {
	schema, err := s.Schema(ctx, table)
	if err != nil {
		return nil, err
	}
	return cursorColumnsFromSchema(schema), nil
}

func cursorColumnsFromSchema(schema rowmodel.Schema) []filament.CursorColumn {
	pk := make(map[string]bool, len(schema.PrimaryKey))
	for _, name := range schema.PrimaryKey {
		pk[name] = true
	}
	priority := make(map[string]int, len(cursorNamePriority))
	for i, name := range cursorNamePriority {
		priority[name] = i + 1
	}

	out := make([]filament.CursorColumn, 0, len(schema.Fields))
	best := 0
	for _, field := range schema.Fields {
		rank := priority[strings.ToLower(field.Name)]
		eligible := isTimestampCursorType(field.Native)
		warning := ""
		if eligible && field.Nullable {
			warning = "NULL cursor values cannot advance a durable watermark"
		}
		if eligible && rank == 0 {
			warning = joinCursorWarning(warning, "Use only if this timestamp advances on every insert and update")
		}
		out = append(out, filament.CursorColumn{
			SchemaField:      field,
			PrimaryKey:       pk[field.Name],
			Eligible:         eligible,
			Rank:             rank,
			Configurable:     true,
			SupportsLookback: eligible,
			Warning:          warning,
		})
		if eligible && rank > 0 && (best == 0 || rank < best) {
			best = rank
		}
	}
	for i := range out {
		out[i].Recommended = out[i].Eligible && out[i].Rank > 0 && out[i].Rank == best
	}
	return out
}

func isTimestampCursorType(native string) bool {
	t := strings.ToLower(strings.TrimSpace(native))
	if end := strings.IndexByte(t, '('); end >= 0 {
		t = strings.TrimSpace(t[:end])
	}
	return t == "datetime" || t == "timestamp"
}

func joinCursorWarning(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}
