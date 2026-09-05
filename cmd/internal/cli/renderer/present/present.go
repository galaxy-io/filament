// Package present turns list results into columns and rows of formatted
// cells. Every renderer reads the same rows, so a value is formatted in one
// place and a column is added in one place.
package present

import (
	"strconv"
	"strings"
	"time"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/cmd/internal/cli/style"
)

// Column is a table heading and the role its cells render with.
type Column struct {
	Title string
	Role  style.Role
}

// Row is one item's cells, in column order, plus the key that names it.
type Row struct {
	Key   string
	Cells []string
}

// Titles returns the column headings.
func Titles(columns []Column) []string {
	titles := make([]string, 0, len(columns))
	for _, column := range columns {
		titles = append(titles, column.Title)
	}
	return titles
}

// Cells returns the rows' cells without their keys.
func Cells(rows []Row) [][]string {
	cells := make([][]string, 0, len(rows))
	for _, row := range rows {
		cells = append(cells, row.Cells)
	}
	return cells
}

// Keys returns the rows' keys.
func Keys(rows []Row) []string {
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.Key)
	}
	return keys
}

// ConnectionColumns are the headings ConnectionRows fill.
func ConnectionColumns() []Column {
	return []Column{
		{Title: "Name", Role: style.RolePrimary},
		{Title: "Connector", Role: style.RoleSecondary},
		{Title: "Replication", Role: style.RoleSecondary},
		{Title: "Used by"},
		{Title: "Updated", Role: style.RoleSecondary},
	}
}

// ConnectionRows formats saved connections; usedBy maps a name to the
// pipelines routing through it.
func ConnectionRows(items []model.ConnectionSummary, usedBy map[string][]string) []Row {
	rows := make([]Row, 0, len(items))
	for _, item := range items {
		rows = append(rows, Row{Key: item.Name, Cells: []string{
			item.Name, item.Connector, TitleCase(Dash(item.Replication)),
			Dash(strings.Join(usedBy[item.Name], ", ")), Stamp(item.UpdatedAt),
		}})
	}
	return rows
}

// PipelineColumns are the headings PipelineRows fill.
func PipelineColumns() []Column {
	return []Column{
		{Title: "Name", Role: style.RolePrimary},
		{Title: "Source", Role: style.RoleSecondary},
		{Title: "Sink", Role: style.RoleSecondary},
		{Title: "Resources", Role: style.RoleNumber},
		{Title: "Sync", Role: style.RoleSecondary},
		{Title: "Write", Role: style.RoleSecondary},
		{Title: "Schedule", Role: style.RoleSecondary},
		{Title: "Status", Role: style.RoleStatus},
		{Title: "Last run", Role: style.RoleSecondary},
	}
}

// PipelineRows formats saved pipelines.
func PipelineRows(items []model.PipelineSummary) []Row {
	rows := make([]Row, 0, len(items))
	for _, item := range items {
		resources := strconv.Itoa(item.ResourceCount)
		if item.AllResources {
			resources = "All"
		}
		rows = append(rows, Row{Key: item.Name, Cells: []string{
			item.Name, item.Source, item.Sink, resources,
			TitleCase(Dash(item.SyncMode)), TitleCase(Dash(item.WriteMode)),
			Dash(item.Schedule), Status(item.LastRunStatus), LastRun(item.LastRunAt),
		}})
	}
	return rows
}

// RunColumns are the headings RunRows fill.
func RunColumns() []Column {
	return []Column{
		{Title: "Run", Role: style.RolePrimary},
		{Title: "Pipeline"},
		{Title: "Version", Role: style.RoleSecondary},
		{Title: "Status", Role: style.RoleStatus},
		{Title: "Records", Role: style.RoleNumber},
		{Title: "Volume", Role: style.RoleNumber},
		{Title: "Started", Role: style.RoleSecondary},
		{Title: "Duration", Role: style.RoleNumber},
	}
}

// RunRows formats run history. The pipeline cell is painted here, name and
// deleted tag as separate segments, so its column carries no role for the
// grid to restyle.
func RunRows(p style.Painter, items []model.RunSummary) []Row {
	rows := make([]Row, 0, len(items))
	for _, run := range items {
		duration := "–"
		if !run.StartedAt.IsZero() && !run.EndedAt.IsZero() {
			duration = style.Elapsed(run.EndedAt.Sub(run.StartedAt))
		}
		pipeline := p.Label(run.Pipeline)
		if run.PipelineDeleted {
			pipeline += " " + p.Status("failed", "[Deleted]")
		}
		rows = append(rows, Row{Key: run.ID, Cells: []string{
			run.ID, pipeline, Dash(run.Version), Status(run.Status),
			style.Count(run.Records), style.Bytes(run.Bytes), Stamp(run.StartedAt), duration,
		}})
	}
	return rows
}

// LastRun renders how long ago the last run started, or a dash when nothing
// has run.
func LastRun(at time.Time) string {
	if at.IsZero() {
		return "–"
	}
	return Ago(at)
}

// Status renders a status with its dot, or a dash when unset.
func Status(value string) string {
	if value == "" {
		return "–"
	}
	return "● " + TitleCase(value)
}

// Stamp renders a timestamp in local time, or a dash when unset.
func Stamp(t time.Time) string {
	if t.IsZero() {
		return "–"
	}
	return t.Local().Format("2006-01-02 15:04")
}

// Ago renders how long ago t was, coarsely.
func Ago(t time.Time) string {
	elapsed := time.Since(t)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return strconv.Itoa(int(elapsed.Minutes())) + "m ago"
	case elapsed < 24*time.Hour:
		return strconv.Itoa(int(elapsed.Hours())) + "h ago"
	default:
		return strconv.Itoa(int(elapsed.Hours()/24)) + "d ago"
	}
}

// TitleCase capitalizes an enumerated value for display.
func TitleCase(value string) string {
	if value == "" || value == "–" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

// Dash stands in for an empty value.
func Dash(value string) string {
	if value == "" {
		return "–"
	}
	return value
}
