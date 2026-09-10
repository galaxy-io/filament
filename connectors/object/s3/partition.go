package s3

import (
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/galaxy-io/filament"
)

const defaultPartition = "dt={{.Date}}"

// partitionData is what a partition template can reference.
type partitionData struct {
	Resource  string
	Run       string
	Date      string
	StartedAt time.Time
}

func newPartitionData(resource string, run filament.RunID, startedAt time.Time) partitionData {
	startedAt = startedAt.UTC()
	return partitionData{Resource: resource, Run: string(run), Date: startedAt.Format(time.DateOnly), StartedAt: startedAt}
}

// parsePartition compiles a partition template and proves it renders a clean
// relative path, so template mistakes surface at config time rather than at
// commit.
func parsePartition(text string) (*template.Template, error) {
	tmpl, err := template.New("partition").Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, err
	}
	if _, err := renderPartition(tmpl, newPartitionData("resource", "run", time.Unix(0, 0))); err != nil {
		return nil, err
	}
	return tmpl, nil
}

// renderPartition returns the partition directories for one resource object.
// Empty means the object sits directly under the resource root.
func renderPartition(tmpl *template.Template, data partitionData) (string, error) {
	var out strings.Builder
	if err := tmpl.Execute(&out, data); err != nil {
		return "", err
	}
	rendered := out.String()
	if rendered == "" {
		return "", nil
	}
	for _, segment := range strings.Split(rendered, "/") {
		switch segment {
		case "", ".", "..":
			return "", fmt.Errorf("partition renders invalid path %q", rendered)
		}
	}
	return rendered, nil
}
