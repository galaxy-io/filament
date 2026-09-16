package object

import (
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/galaxy-io/filament"
)

const (
	// DefaultPartition places resource objects in one UTC date directory.
	DefaultPartition = "dt={{.Date}}"
	runsDir          = "_runs"
	successMarker    = "_SUCCESS.json"
)

type partitionData struct {
	Resource  string
	Run       string
	Date      string
	StartedAt time.Time
}

// ParsePartition compiles and validates a resource partition template.
func ParsePartition(text string) (*template.Template, error) {
	tmpl, err := template.New("partition").Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, err
	}
	if _, err := renderPartition(tmpl, newPartitionData("resource", "run", time.Unix(0, 0))); err != nil {
		return nil, err
	}
	return tmpl, nil
}

func newPartitionData(resource string, run filament.RunID, startedAt time.Time) partitionData {
	startedAt = startedAt.UTC()
	return partitionData{Resource: resource, Run: string(run), Date: startedAt.Format(time.DateOnly), StartedAt: startedAt}
}

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

// Layout places one run's resource objects and its success marker.
type Layout struct {
	prefix    string
	run       filament.RunID
	startedAt time.Time
	partition *template.Template
	extension string
}

// NewLayout creates an immutable object layout for a run.
func NewLayout(prefix string, partition *template.Template, extension string, run filament.RunSpec) Layout {
	startedAt := run.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	return Layout{prefix: prefix, run: run.Run, startedAt: startedAt, partition: partition, extension: extension}
}

// Object returns the key for a resource object.
func (l Layout) Object(resource string) (string, error) {
	partition, err := renderPartition(l.partition, newPartitionData(resource, l.run, l.startedAt))
	if err != nil {
		return "", err
	}
	if partition == "" {
		return l.join(resource, string(l.run)+l.extension), nil
	}
	return l.join(resource, partition, string(l.run)+l.extension), nil
}

// Success returns the run success-marker key.
func (l Layout) Success() string { return l.join(runsDir, string(l.run), successMarker) }

// Run returns the run identifier captured by the layout.
func (l Layout) Run() filament.RunID { return l.run }

func (l Layout) join(parts ...string) string {
	if l.prefix != "" {
		parts = append([]string{l.prefix}, parts...)
	}
	return strings.Join(parts, "/")
}
