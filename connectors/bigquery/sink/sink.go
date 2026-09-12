// Package bigquery implements the Google BigQuery sink.
package bigquery

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"cloud.google.com/go/bigquery"

	"github.com/galaxy-io/filament"
	bigqueryconnection "github.com/galaxy-io/filament/connectors/bigquery/internal/connection"
)

const sinkName = "bigquery"

// Sink owns the BigQuery client and the typed tables prepared for a run.
type Sink struct {
	client   *bigquery.Client
	run      filament.RunID
	project  string
	dataset  string
	location string
	policies map[string]filament.WritePolicy
	tables   map[string]tableDefinition

	stageMu sync.Mutex
	stages  map[string]struct{}
}

// New returns an unconfigured BigQuery sink.
func New() *Sink { return &Sink{} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.Schematized       = (*Sink)(nil)
)

// Name identifies this sink implementation.
func (*Sink) Name() string { return sinkName }

// Validate checks connection syntax without accessing the network.
func (*Sink) Validate(cfg filament.Config) error {
	if _, err := bigqueryconnection.Resolve(cfg); err != nil {
		return fmt.Errorf("bigquery sink: connection config: %w", err)
	}
	return nil
}

// TestConnection authenticates with BigQuery and executes a trivial query.
func (*Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	resolved, err := bigqueryconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("bigquery sink: connection config: %w", err)
	}
	if err := bigqueryconnection.Test(ctx, resolved); err != nil {
		return fmt.Errorf("bigquery sink: %w", err)
	}
	return nil
}

// Open establishes the run's BigQuery client and ensures its destination
// dataset exists. Resource tables are created later by EnsureSchema.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	if s.client != nil {
		return fmt.Errorf("bigquery sink: already open")
	}
	cfg := filament.NewConfig(run.Sink.Config)
	resolved, err := bigqueryconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("bigquery sink: connection config: %w", err)
	}
	dataset := strings.TrimSpace(cfg.String("dataset"))
	ddl, err := createDatasetDDL(resolved.ProjectID, dataset, resolved.Location)
	if err != nil {
		return fmt.Errorf("bigquery sink: dataset: %w", err)
	}
	client, err := bigqueryconnection.Open(ctx, resolved)
	if err != nil {
		return fmt.Errorf("bigquery sink: open: %w", err)
	}
	s.client = client
	s.run = run.Run
	s.project = resolved.ProjectID
	s.dataset = dataset
	s.location = resolved.Location
	s.policies = run.WritePolicies
	s.tables = make(map[string]tableDefinition)
	s.stages = make(map[string]struct{})
	if err := s.execute(ctx, ddl); err != nil {
		s.release()
		return fmt.Errorf("bigquery sink: create dataset %s: %w", qualified(s.project, s.dataset), err)
	}
	metadata, err := client.DatasetInProject(s.project, s.dataset).Metadata(ctx)
	if err != nil {
		s.release()
		return fmt.Errorf("bigquery sink: inspect dataset %s: %w", qualified(s.project, s.dataset), err)
	}
	s.location = metadata.Location
	s.client.Location = metadata.Location
	return nil
}

// Commit atomically promotes each staged full replacement, then closes the
// client. Append and keyed writes are durable when their Apply jobs complete.
func (s *Sink) Commit(ctx context.Context) error {
	if s.client == nil {
		return fmt.Errorf("bigquery sink: commit before open")
	}
	resources := make([]string, 0, len(s.tables))
	for resource := range s.tables {
		resources = append(resources, resource)
	}
	sort.Strings(resources)
	for _, resource := range resources {
		table := s.tables[resource]
		if table.replacement == "" {
			continue
		}
		if err := s.promoteReplacement(ctx, table); err != nil {
			return fmt.Errorf("bigquery sink: promote replacement for %q: %w", resource, err)
		}
		s.deleteStage(context.WithoutCancel(ctx), table.replacement)
	}
	s.release()
	return nil
}

// Abort removes run-scoped staging tables and closes the client. Additive
// destination schema changes are intentionally retained.
func (s *Sink) Abort(ctx context.Context) error {
	if s.client == nil {
		return nil
	}
	cleanup := context.WithoutCancel(ctx)
	for _, stage := range s.stageIDs() {
		s.deleteStage(cleanup, stage)
	}
	s.release()
	return nil
}

func (s *Sink) execute(ctx context.Context, statement string) error {
	query := s.client.Query(statement)
	if s.location != "" {
		query.Location = s.location
	}
	job, err := query.Run(ctx)
	if err != nil {
		return err
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return err
	}
	return status.Err()
}

func (s *Sink) release() {
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
	s.run = ""
	s.project = ""
	s.dataset = ""
	s.location = ""
	s.policies = nil
	s.tables = nil
	s.stageMu.Lock()
	s.stages = nil
	s.stageMu.Unlock()
}

func (s *Sink) modeFor(resource string) filament.WriteMode {
	policy, ok := s.policies[resource]
	if !ok {
		policy = s.policies[""]
	}
	return policy.Capability.Mode
}

func (s *Sink) trackStage(stage string) {
	s.stageMu.Lock()
	defer s.stageMu.Unlock()
	s.stages[stage] = struct{}{}
}

func (s *Sink) untrackStage(stage string) {
	s.stageMu.Lock()
	defer s.stageMu.Unlock()
	delete(s.stages, stage)
}

func (s *Sink) stageIDs() []string {
	s.stageMu.Lock()
	defer s.stageMu.Unlock()
	stages := make([]string, 0, len(s.stages))
	for stage := range s.stages {
		stages = append(stages, stage)
	}
	return stages
}

func (s *Sink) deleteStage(ctx context.Context, stage string) {
	if s.client == nil {
		return
	}
	if err := s.client.DatasetInProject(s.project, s.dataset).Table(stage).Delete(ctx); err == nil {
		s.untrackStage(stage)
	}
}
