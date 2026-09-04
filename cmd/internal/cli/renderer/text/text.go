// Package text renders target-neutral CLI results as plain terminal text.
package text

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/cmd/internal/cli/renderer/present"
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
		rows = append(rows, []string{marker, item.Name, present.TitleCase(item.Kind), item.Location})
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
	title := strings.ToUpper(result.Kind[:1]) + result.Kind[1:] + "s in"
	return page(w, title, location, present.ConnectionColumns(), present.ConnectionRows(result.Items, usedBy), result.Page)
}

// Pipelines renders saved pipelines.
func Pipelines(w io.Writer, result model.PipelineList, location string) error {
	if len(result.Items) == 0 {
		_, err := fmt.Fprintln(w, "No saved pipelines.")
		return err
	}
	return page(w, "Pipelines in", location, present.PipelineColumns(), present.PipelineRows(result.Items), result.Page)
}

// Runs renders one page of run history.
func Runs(w io.Writer, result model.RunList) error {
	if len(result.Items) == 0 {
		_, err := fmt.Fprintln(w, "No runs yet.")
		return err
	}
	subject := "all pipelines"
	if result.Pipeline != "" {
		subject = result.Pipeline
	}
	return page(w, "Runs of", subject, present.RunColumns(), present.RunRows(result.Items), result.Page)
}

// page renders one titled table and its paging footer.
func page(w io.Writer, prefix, subject string, columns []present.Column, rows []present.Row, info model.PageInfo) error {
	p := style.New(w)
	styled := make([]style.Column, 0, len(columns))
	for _, column := range columns {
		styled = append(styled, style.Column{Title: column.Title, Role: column.Role})
	}
	if err := table(w, p.Title(prefix, subject), p.Table(styled, present.Cells(rows))); err != nil {
		return err
	}
	return pageFooter(w, p, len(rows), info)
}

// pageFooter says how much of the collection this page shows and how to get
// the next one. A target that does not count reports only the page.
func pageFooter(w io.Writer, p style.Painter, shown int, page model.PageInfo) error {
	if page.NextCursor == "" && page.PreviousCursor == "" {
		return nil
	}
	summary := fmt.Sprintf("Showing %d", shown)
	if page.Total > 0 {
		summary = fmt.Sprintf("Showing %d of %d", shown, page.Total)
	}
	if page.NextCursor != "" {
		summary += ". Next page: --next " + page.NextCursor
	}
	_, err := fmt.Fprintln(w, p.Muted(summary))
	return err
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
		rows = append(rows, []string{resource.Name, present.Dash(display), selectable, present.Dash(strings.Join(resource.PrimaryKey, ", ")), estimated})
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
