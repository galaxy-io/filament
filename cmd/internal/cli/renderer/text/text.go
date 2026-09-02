// Package text renders target-neutral CLI results as plain terminal text.
package text

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/cmd/internal/cli/style"
)

// Contexts renders configured contexts stored at location.
func Contexts(w io.Writer, result model.ContextList, location string) error {
	p := style.New(w)
	rows := make([][]string, 0, len(result.Items))
	for _, item := range result.Items {
		marker := " "
		if item.Current {
			marker = p.Accent("●")
		}
		rows = append(rows, []string{marker, item.Name, titleCase(item.Kind), item.Location})
	}
	columns := []style.Column{{Title: " "}, {Title: "Name", Role: style.RolePrimary}, {Title: "Kind", Role: style.RoleSecondary}, {Title: "Location"}}
	return table(w, p.Title("Contexts in", location), p.Table(columns, rows))
}

// CurrentContext renders the effective context name.
func CurrentContext(w io.Writer, name string) error {
	_, err := fmt.Fprintln(w, name)
	return err
}

// Connections renders saved connections and the pipelines that use them.
func Connections(w io.Writer, result model.ConnectionList, location string, usedBy map[string][]string) error {
	if len(result.Items) == 0 {
		_, err := fmt.Fprintf(w, "No saved %ss.\n", result.Kind)
		return err
	}
	p := style.New(w)
	rows := make([][]string, 0, len(result.Items))
	for _, connection := range result.Items {
		row := []string{connection.Name, connection.Connector}
		if result.Kind == "source" {
			row = append(row, titleCase(dash(connection.Replication)))
		}
		row = append(row,
			dash(strings.Join(usedBy[connection.Name], ", ")), stamp(connection.CreatedAt), stamp(connection.UpdatedAt),
		)
		rows = append(rows, row)
	}
	columns := []style.Column{
		{Title: "Name", Role: style.RolePrimary},
		{Title: "Connector", Role: style.RoleSecondary},
	}
	if result.Kind == "source" {
		columns = append(columns, style.Column{Title: "Replication", Role: style.RoleSecondary})
	}
	columns = append(columns,
		style.Column{Title: "Used by"},
		style.Column{Title: "Created", Role: style.RoleMuted},
		style.Column{Title: "Updated", Role: style.RoleMuted},
	)
	title := strings.ToUpper(result.Kind[:1]) + result.Kind[1:] + "s in"
	if err := table(w, p.Title(title, location), p.Table(columns, rows)); err != nil {
		return err
	}
	return pageFooter(w, len(result.Items), result.Total, "filament "+result.Kind+" list", result.NextCursor)
}

// Pipelines renders saved pipelines.
func Pipelines(w io.Writer, result model.PipelineList, location string) error {
	if len(result.Items) == 0 {
		_, err := fmt.Fprintln(w, "No saved pipelines.")
		return err
	}
	p := style.New(w)
	rows := make([][]string, 0, len(result.Items))
	for _, pipeline := range result.Items {
		resources := strconv.Itoa(pipeline.ResourceCount)
		if pipeline.AllResources {
			resources = "all"
		}
		rows = append(rows, []string{
			pipeline.Name, pipeline.Source, pipeline.Sink, titleCase(resources),
			titleCase(dash(pipeline.SyncMode)), titleCase(dash(pipeline.WriteMode)),
			dash(pipeline.Schedule), lastRun(pipeline.LastRunStatus, pipeline.LastRunAt), stamp(pipeline.UpdatedAt),
		})
	}
	columns := []style.Column{
		{Title: "Name", Role: style.RolePrimary},
		{Title: "Source", Role: style.RoleSecondary},
		{Title: "Sink", Role: style.RoleSecondary},
		{Title: "Resources", Role: style.RoleNumber},
		{Title: "Sync", Role: style.RoleSecondary},
		{Title: "Write", Role: style.RoleSecondary},
		{Title: "Schedule", Role: style.RoleSecondary},
		{Title: "Last run", Role: style.RoleSecondary},
		{Title: "Updated", Role: style.RoleMuted},
	}
	if err := table(w, p.Title("Pipelines in", location), p.Table(columns, rows)); err != nil {
		return err
	}
	return pageFooter(w, len(result.Items), result.Total, "filament pipeline list", result.NextCursor)
}

// Runs renders target run history newest first.
func Runs(w io.Writer, result model.RunList, target string) error {
	if len(result.Items) == 0 {
		_, err := fmt.Fprintln(w, "No runs.")
		return err
	}
	p := style.New(w)
	title, body := RunsTable(p, result, target)
	if err := table(w, title, body); err != nil {
		return err
	}
	command := "filament run list"
	if result.Pipeline != "" {
		command += " " + result.Pipeline
	}
	return pageFooter(w, len(result.Items), result.Total, command, result.NextCursor)
}

func pageFooter(w io.Writer, shown, total int, command, next string) error {
	if total < shown {
		total = shown
	}
	if _, err := fmt.Fprintf(w, "\n> Showing %d of %d.\n", shown, total); err != nil {
		return err
	}
	if next == "" {
		return nil
	}
	_, err := fmt.Fprintf(w, "> To display the next page, run `%s --next %s`\n", command, next)
	return err
}

// RunsTable builds the runs table with a caller-supplied painter so
// interactive surfaces keep their styling. It returns the title line and the
// rendered table body.
func RunsTable(p style.Painter, result model.RunList, target string) (string, string) {
	rows := make([][]string, 0, len(result.Items))
	for _, run := range result.Items {
		duration := "–"
		if !run.StartedAt.IsZero() && !run.EndedAt.IsZero() {
			duration = style.Elapsed(run.EndedAt.Sub(run.StartedAt))
		}
		rows = append(rows, []string{
			run.Pipeline, dash(run.Version), titleCase(run.Status), style.Count(run.Records),
			style.Bytes(run.Bytes), stamp(run.StartedAt), duration,
		})
	}
	columns := []style.Column{
		{Title: "Pipeline", Role: style.RolePrimary},
		{Title: "Version", Role: style.RoleSecondary},
		{Title: "Status", Role: style.RoleSecondary},
		{Title: "Records", Role: style.RoleNumber},
		{Title: "Volume", Role: style.RoleNumber},
		{Title: "Started", Role: style.RoleMuted},
		{Title: "Duration", Role: style.RoleNumber},
	}
	return p.Title("Runs on", target), p.Table(columns, rows)
}

// TitleCase capitalizes an enum-style value for table display.
func TitleCase(value string) string { return titleCase(value) }

// Stamp renders a target-owned timestamp, dash when the target has none.
func Stamp(value time.Time) string { return stamp(value) }

// LastRunCell renders a last-run status with how long ago it started.
func LastRunCell(status string, at time.Time) string { return lastRun(status, at) }

// lastRun combines a run status with how long ago it started.
func lastRun(status string, at time.Time) string {
	if status == "" {
		return "–"
	}
	if at.IsZero() {
		return titleCase(status)
	}
	return titleCase(status) + " · " + ago(at)
}

// ago renders a coarse relative time for list surfaces.
func ago(value time.Time) string {
	elapsed := time.Since(value)
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

// stamp renders a target-owned timestamp, blank when the target has none.
func stamp(value time.Time) string {
	if value.IsZero() {
		return "–"
	}
	return value.Local().Format("2006-01-02 15:04")
}

// Resources renders discovered resources and how long discovery took.
func Resources(w io.Writer, result model.ResourceList, elapsed time.Duration) error {
	if len(result.Items) == 0 {
		_, err := fmt.Fprintf(w, "No resources discovered for source %q.\n", result.Source)
		return err
	}
	p := style.New(w)
	rows := make([][]string, 0, len(result.Items))
	for _, resource := range result.Items {
		display := ""
		if resource.DisplayName != resource.Name {
			display = resource.DisplayName
		}
		estimated := "–"
		if resource.EstimatedRows > 0 {
			estimated = style.Count(resource.EstimatedRows)
		}
		selectable := p.Muted("–")
		if resource.Selectable {
			selectable = p.Success("✓")
		}
		rows = append(rows, []string{resource.Name, dash(display), selectable, dash(strings.Join(resource.PrimaryKey, ", ")), estimated})
	}
	columns := []style.Column{
		{Title: "Resource", Role: style.RolePrimary},
		{Title: "Display name", Role: style.RoleSecondary},
		{Title: "Selectable"},
		{Title: "Primary key", Role: style.RoleSecondary},
		{Title: "Est. rows", Role: style.RoleNumber},
	}
	title := p.Title("Resources from", result.Source) + " " + p.Muted("["+style.Elapsed(elapsed)+"]")
	return table(w, title, p.Table(columns, rows))
}

func table(w io.Writer, title, grid string) error {
	_, err := fmt.Fprintf(w, "%s\n\n%s", title, grid)
	return err
}

// titleCase capitalizes an enumerated value for display only.
func titleCase(value string) string {
	if value == "" || value == "–" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func dash(value string) string {
	if value == "" {
		return "–"
	}
	return value
}
