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
		rows = append(rows, []string{connection.Name, connection.Connector, dash(strings.Join(usedBy[connection.Name], ", "))})
	}
	columns := []style.Column{{Title: "Name", Role: style.RolePrimary}, {Title: "Connector", Role: style.RoleSecondary}, {Title: "Used by"}}
	title := strings.ToUpper(result.Kind[:1]) + result.Kind[1:] + "s in"
	return table(w, p.Title(title, location), p.Table(columns, rows))
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
		rows = append(rows, []string{pipeline.Name, pipeline.Source, pipeline.Sink, titleCase(resources), titleCase(dash(pipeline.SyncMode)), titleCase(dash(pipeline.WriteMode))})
	}
	columns := []style.Column{
		{Title: "Name", Role: style.RolePrimary},
		{Title: "Source", Role: style.RoleSecondary},
		{Title: "Sink", Role: style.RoleSecondary},
		{Title: "Resources", Role: style.RoleNumber},
		{Title: "Sync", Role: style.RoleSecondary},
		{Title: "Write", Role: style.RoleSecondary},
	}
	return table(w, p.Title("Pipelines in", location), p.Table(columns, rows))
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
