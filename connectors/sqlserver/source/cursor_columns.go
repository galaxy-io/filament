package sqlserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
)

var cursorNamePriority = []string{
	"updated_at", "modified_at", "last_modified_at", "last_updated_at", "updated", "modified",
	"modifieddate", "lastmodified", "updatedon", "modifiedon",
}

// CursorColumns reports every column of a table and recommends the one an
// incremental read should follow: a conventionally named timestamp, or else a
// rowversion column.
func (s *Source) CursorColumns(ctx context.Context, name string) ([]filament.CursorColumn, error) {
	t, err := s.tableMeta(ctx, name)
	if err != nil {
		return nil, err
	}
	explicit := ""
	if s.cursorColumns != nil {
		explicit = s.cursorColumns[name]
	}
	return cursorColumns(t, explicit), nil
}

// column finds a configured column by name. An exact match wins, since a
// case-sensitive catalog can hold names that differ only in case; otherwise a
// name matching one column regardless of case finds it.
func (t *table) column(name string) (column, error) {
	var folded []column
	for _, c := range t.cols {
		if c.name == name {
			return c, nil
		}
		if strings.EqualFold(c.name, name) {
			folded = append(folded, c)
		}
	}
	switch len(folded) {
	case 0:
		return column{}, fmt.Errorf("column %q does not exist", name)
	case 1:
		return folded[0], nil
	default:
		return column{}, fmt.Errorf("column %q matches %d columns that differ only in case; use its exact name", name, len(folded))
	}
}

func cursorColumns(t *table, explicit string) []filament.CursorColumn {
	pk := make(map[string]bool, len(t.pks))
	for _, c := range t.pks {
		pk[c.name] = true
	}
	// Rank 1 is reserved for the configured cursor, so it is the only
	// recommendation even when a conventionally named column also exists.
	priority := make(map[string]int, len(cursorNamePriority))
	for i, name := range cursorNamePriority {
		priority[name] = i + 2
	}
	rowversionRank := len(cursorNamePriority) + 2
	if c, err := t.column(explicit); err == nil {
		explicit = c.name
	}

	out := make([]filament.CursorColumn, len(t.cols))
	best := 0
	for i, c := range t.cols {
		rank := priority[strings.ToLower(c.name)]
		rowversion := isRowversion(c.decl())
		if rowversion {
			rank = rowversionRank
		}
		if c.name == explicit {
			rank = 1
		}
		eligible := cursorEligible(c)
		warning := ""
		switch {
		case isCursorType(c.decl()) && c.nullable:
			warning = "Durable cursor columns must be NOT NULL"
		case eligible && !rowversion && priority[strings.ToLower(c.name)] == 0:
			warning = "Use only if this timestamp advances on every insert and update"
		}
		out[i] = filament.CursorColumn{
			SchemaField:      t.schema.Fields[i],
			PrimaryKey:       pk[c.name],
			Eligible:         eligible,
			Rank:             rank,
			Configurable:     true,
			SupportsLookback: eligible && !rowversion,
			Warning:          warning,
		}
		if eligible && rank > 0 && (best == 0 || rank < best) {
			best = rank
		}
	}
	for i := range out {
		out[i].Recommended = out[i].Eligible && out[i].Rank > 0 && out[i].Rank == best
	}
	return out
}

func cursorEligible(c column) bool { return isCursorType(c.decl()) && !c.nullable }

func isCursorType(decl string) bool {
	switch {
	case isRowversion(decl), decl == "datetime", decl == "smalldatetime",
		strings.HasPrefix(decl, "datetime2"), strings.HasPrefix(decl, "datetimeoffset"):
		return true
	}
	return false
}
