package s3

import (
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
)

const (
	runsDir       = "_runs"
	successMarker = "_SUCCESS.json"
)

// keyLayout places one run's objects. The partition template renders from the
// run's start time so every object of a run shares one partition; the run id
// is always the filename so runs never collide; manifests sit beside the
// resources under a directory query engines skip.
type keyLayout struct {
	prefix    string
	run       filament.RunID
	startedAt time.Time
	partition *template.Template
	extension string
}

func newKeyLayout(cfg sinkConfig, run filament.RunSpec) keyLayout {
	startedAt := run.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	options := encoder.Options{FileFormat: cfg.fileFormat, Compression: cfg.compression}
	return keyLayout{prefix: cfg.prefix, run: run.Run, startedAt: startedAt, partition: cfg.partition, extension: options.Extension()}
}

func (l keyLayout) object(resource string) (string, error) {
	partition, err := renderPartition(l.partition, newPartitionData(resource, l.run, l.startedAt))
	if err != nil {
		return "", fmt.Errorf("s3 sink: %w", err)
	}
	if partition == "" {
		return l.join(resource, string(l.run)+l.extension), nil
	}
	return l.join(resource, partition, string(l.run)+l.extension), nil
}

func (l keyLayout) success() string {
	return l.join(runsDir, string(l.run), successMarker)
}

func (l keyLayout) join(parts ...string) string {
	if l.prefix != "" {
		parts = append([]string{l.prefix}, parts...)
	}
	return strings.Join(parts, "/")
}
