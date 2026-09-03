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

// Renderer writes results in one table layout.
type Renderer struct {
	w      io.Writer
	layout style.Layout
}

// New binds a renderer to its output and layout.
func New(w io.Writer, layout style.Layout) Renderer {
	return Renderer{w: w, layout: layout}
}

// Contexts renders configured contexts stored at location.
func (r Renderer) Contexts(result model.ContextList, location string) error {
	w := r.w
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
	return table(w, p.Title("Contexts in", location), p.Grid(r.layout, columns, rows))
}

// CurrentContext renders the effective context name.
func CurrentContext(w io.Writer, name string) error {
	_, err := fmt.Fprintln(w, name)
	return err
}

// Connections renders saved connections and the pipelines that use them.
// command is the invocation the footer completes for the next page.
func (r Renderer) Connections(result model.ConnectionList, location string, usedBy map[string][]string, command string) error {
	if len(result.Items) == 0 {
		_, err := fmt.Fprintf(r.w, "No saved %ss.\n", result.Kind)
		return err
	}
	title := strings.ToUpper(result.Kind[:1]) + result.Kind[1:] + "s in"
	return r.page(title, location, present.ConnectionColumns(), present.ConnectionRows(result.Items, usedBy), result.Page, command)
}

// Pipelines renders saved pipelines.
func (r Renderer) Pipelines(result model.PipelineList, location, command string) error {
	if len(result.Items) == 0 {
		_, err := fmt.Fprintln(r.w, "No saved pipelines.")
		return err
	}
	return r.page("Pipelines in", location, present.PipelineColumns(), present.PipelineRows(result.Items), result.Page, command)
}

// Runs renders one page of run history.
func (r Renderer) Runs(result model.RunList, command string) error {
	if len(result.Items) == 0 {
		_, err := fmt.Fprintln(r.w, "No runs yet.")
		return err
	}
	subject := "all pipelines"
	if result.Pipeline != "" {
		subject = result.Pipeline
	}
	return r.page("Runs of", subject, present.RunColumns(), present.RunRows(result.Items), result.Page, command)
}

// page renders one titled table and its paging footer.
func (r Renderer) page(prefix, subject string, columns []present.Column, rows []present.Row, info model.PageInfo, command string) error {
	p := style.New(r.w)
	styled := make([]style.Column, 0, len(columns))
	for _, column := range columns {
		styled = append(styled, style.Column{Title: column.Title, Role: column.Role})
	}
	if err := table(r.w, p.Title(prefix, subject), p.Grid(r.layout, styled, present.Cells(rows))); err != nil {
		return err
	}
	return pageFooter(r.w, p, len(rows), info, command)
}

// pageFooter is one line under the table: how much of the collection this
// page shows, and the command that shows the next page. A target that does
// not count reports only the next page.
func pageFooter(w io.Writer, p style.Painter, shown int, page model.PageInfo, command string) error {
	if page.NextCursor == "" && page.PreviousCursor == "" {
		return nil
	}
	parts := []string{}
	if page.Total > 0 {
		parts = append(parts, fmt.Sprintf("Showing %d of %d.", shown, page.Total))
	}
	if page.NextCursor != "" {
		parts = append(parts, "To display the next page, run "+p.Bold("`"+command+" --next "+page.NextCursor+"`"))
	}
	_, err := fmt.Fprintf(w, "\n%s %s\n", p.Accent(">"), strings.Join(parts, " "))
	return err
}

// Resources renders discovered resources and how long discovery took.
func (r Renderer) Resources(result model.ResourceList, elapsed time.Duration) error {
	w := r.w
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
	return table(w, title, p.Grid(r.layout, columns, rows))
}

func table(w io.Writer, title, grid string) error {
	_, err := fmt.Fprintf(w, "%s\n\n%s", title, grid)
	return err
}
