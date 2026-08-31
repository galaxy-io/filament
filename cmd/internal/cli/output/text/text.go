// Package text renders target-neutral CLI results for a plain terminal.
package text

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// Connections renders a connection list as a terminal table.
func Connections(w io.Writer, result model.ConnectionList) error {
	if len(result.Items) == 0 {
		_, err := fmt.Fprintf(w, "No saved %ss.\n", result.Kind)
		return err
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "Name\tConnector\tDescription"); err != nil {
		return err
	}
	for _, connection := range result.Items {
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\n", connection.Name, connection.Connector, connection.Description); err != nil {
			return err
		}
	}
	return table.Flush()
}

// Pipelines renders a pipeline list as a terminal table.
func Pipelines(w io.Writer, result model.PipelineList) error {
	if len(result.Items) == 0 {
		_, err := fmt.Fprintln(w, "No saved pipelines.")
		return err
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "Name\tSource\tSink\tResources\tSync mode\tWrite mode"); err != nil {
		return err
	}
	for _, pipeline := range result.Items {
		resources := strconv.Itoa(pipeline.ResourceCount)
		if pipeline.AllResources {
			resources = "all"
		}
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n",
			pipeline.Name, pipeline.Source, pipeline.Sink, resources, pipeline.SyncMode, pipeline.WriteMode); err != nil {
			return err
		}
	}
	return table.Flush()
}

// Resources renders a discovered resource list as a terminal table.
func Resources(w io.Writer, result model.ResourceList) error {
	if len(result.Items) == 0 {
		_, err := fmt.Fprintf(w, "No resources discovered for source %q.\n", result.Source)
		return err
	}
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "Resource\tDisplay name\tSelectable\tPrimary key\tEstimated rows"); err != nil {
		return err
	}
	for _, resource := range result.Items {
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%d\n",
			resource.Name,
			resource.DisplayName,
			yesNo(resource.Selectable),
			strings.Join(resource.PrimaryKey, ","),
			resource.EstimatedRows,
		); err != nil {
			return err
		}
	}
	return table.Flush()
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
