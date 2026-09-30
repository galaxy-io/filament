// Package bigquery implements the Google BigQuery sink.
package bigquery

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/bigquery"
	storage "cloud.google.com/go/bigquery/storage/apiv1"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/googleapi"

	"github.com/galaxy-io/filament"
	bigqueryconnection "github.com/galaxy-io/filament/connectors/bigquery/internal/connection"
)

const (
	sinkName          = "bigquery"
	uploadConcurrency = 8
	setupConcurrency  = 8
	queuedBytes       = 64 << 20
	stageLifetime     = 7 * 24 * time.Hour
)

// Sink streams Arrow batches into isolated pending streams. Only Commit makes
// rows visible in destination tables; its per-table jobs run concurrently.
type Sink struct {
	client                     *bigquery.Client
	storage                    *storage.BigQueryWriteClient
	run                        filament.RunID
	execution                  string
	attempt                    string
	project, dataset, location string
	policies                   map[string]filament.WritePolicy
	tables                     map[string]*tableState
	ctx                        context.Context
	cancel                     context.CancelFunc
	memory                     *semaphore.Weighted
	uploads                    *semaphore.Weighted
	setups                     *semaphore.Weighted
	workers                    sync.WaitGroup
	sealed                     atomic.Bool
	errMu                      sync.Mutex
	err                        error
}

func New() *Sink { return &Sink{} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.Schematized       = (*Sink)(nil)
)

func (*Sink) Name() string { return sinkName }
func (*Sink) Validate(cfg filament.Config) error {
	_, err := bigqueryconnection.Resolve(cfg)
	return err
}
func (*Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	resolved, err := bigqueryconnection.Resolve(cfg)
	if err != nil {
		return err
	}
	return bigqueryconnection.Test(ctx, resolved)
}

func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	if s.client != nil {
		return fmt.Errorf("bigquery sink: already open")
	}
	cfg := filament.NewConfig(run.Sink.Config)
	resolved, err := bigqueryconnection.Resolve(cfg)
	if err != nil {
		return err
	}
	dataset := strings.TrimSpace(cfg.String("dataset"))
	if !validDataset(dataset) {
		return fmt.Errorf("bigquery sink: invalid dataset")
	}
	if run.Run == "" {
		return fmt.Errorf("bigquery sink: run ID is required for retry-safe publication")
	}
	s.client, err = bigqueryconnection.Open(ctx, resolved)
	if err != nil {
		return err
	}
	s.storage, err = storage.NewBigQueryWriteClient(ctx)
	if err != nil {
		s.release()
		return fmt.Errorf("bigquery sink: storage client: %w", err)
	}
	s.project, s.dataset, s.location = resolved.ProjectID, dataset, resolved.Location
	s.initialize(ctx, run)
	ds := s.client.DatasetInProject(s.project, s.dataset)
	metadata, err := ds.Metadata(ctx)
	if isHTTPCode(err, 404) {
		err = ds.Create(ctx, &bigquery.DatasetMetadata{Location: s.location})
		if err == nil || isHTTPCode(err, 409) {
			metadata, err = ds.Metadata(ctx)
		}
	}
	if err != nil {
		s.release()
		return fmt.Errorf("bigquery sink: ensure dataset: %w", err)
	}
	s.location = metadata.Location
	s.client.Location = metadata.Location
	slog.InfoContext(ctx, "BigQuery Storage Write API ready", "component", "bigquery", "event.name", "bigquery.write.ready", "run_id", s.run, "transport", "storage_write_arrow", "compression", "gzip", "upload_concurrency", uploadConcurrency, "inflight_per_stream", streamInflightRequests, "setup_concurrency", setupConcurrency, "flush_interval_ms", s.Spec().Capabilities.PreferredFlushInterval.Milliseconds(), "queue_bytes", queuedBytes, "location", s.location)
	return nil
}

func (s *Sink) initialize(ctx context.Context, run filament.RunSpec) {
	s.run, s.policies = run.Run, run.WritePolicies
	s.execution = run.ExecutionID
	s.attempt = uuid.NewString()
	s.tables = make(map[string]*tableState)
	s.memory = semaphore.NewWeighted(queuedBytes)
	s.uploads = semaphore.NewWeighted(uploadConcurrency)
	s.setups = semaphore.NewWeighted(setupConcurrency)
	// Extraction cancellation requests a pause, not an upload abort. Commit
	// drains accepted writes with its own context; Abort explicitly cancels them.
	s.ctx, s.cancel = context.WithCancel(context.WithoutCancel(ctx))
	s.sealed.Store(false)
	s.err = nil
}

// Commit joins all uploads before publishing any table. Checkpoints must stay
// pending until every publication succeeds. Publication is atomic per table,
// not across the whole dataset.
func (s *Sink) Commit(ctx context.Context) error {
	if s.client == nil {
		return fmt.Errorf("bigquery sink: commit before open")
	}
	stop := context.AfterFunc(ctx, s.cancel)
	defer stop()
	s.seal()
	s.workers.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.failure(); err != nil {
		return err
	}
	group, commitCtx := errgroup.WithContext(ctx)
	group.SetLimit(uploadConcurrency)
	for _, state := range s.orderedTables() {
		group.Go(func() error {
			if err := s.publish(commitCtx, state); err != nil {
				return fmt.Errorf("bigquery sink: publish %s: %w", state.definition.name, err)
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	s.cleanup()
	s.release()
	return nil
}

func (s *Sink) Abort(context.Context) error {
	if s.client == nil {
		return nil
	}
	s.cancel()
	s.seal()
	s.workers.Wait()
	s.cleanup()
	s.release()
	return nil
}

func (s *Sink) seal() {
	if s.sealed.Swap(true) {
		return
	}
	for _, state := range s.tables {
		if state.queue != nil {
			state.queue.close()
		}
	}
}
func (s *Sink) failure() error { s.errMu.Lock(); defer s.errMu.Unlock(); return s.err }
func (s *Sink) fail(err error) {
	s.errMu.Lock()
	if s.err == nil {
		s.err = err
		s.cancel()
	}
	s.errMu.Unlock()
}
func (s *Sink) modeFor(resource string) filament.WriteMode {
	policy, ok := s.policies[resource]
	if !ok {
		policy = s.policies[""]
	}
	return policy.Capability.Mode
}
func (s *Sink) orderedTables() []*tableState {
	names := make([]string, 0, len(s.tables))
	for name := range s.tables {
		names = append(names, name)
	}
	sort.Strings(names)
	states := make([]*tableState, 0, len(names))
	for _, name := range names {
		states = append(states, s.tables[name])
	}
	return states
}
func (s *Sink) cleanup() {
	// An ambiguous query submission can still be reading its staging table.
	// Preserve those tables for job recovery; their expiration bounds retention.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	group := new(errgroup.Group)
	group.SetLimit(uploadConcurrency)
	for _, state := range s.tables {
		if !state.stageCreated || (state.publishing && !state.published) {
			continue
		}
		group.Go(func() error { return s.client.DatasetInProject(s.project, s.dataset).Table(state.stage).Delete(ctx) })
	}
	_ = group.Wait()
}
func (s *Sink) release() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.storage != nil {
		_ = s.storage.Close()
		s.storage = nil
	}
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
}
func isHTTPCode(err error, code int) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == code
}

// Slow operations are visible at INFO without logging payloads or credentials.
// DEBUG provides all per-batch timings for throughput diagnosis.
func (s *Sink) logTiming(phase, resource string, started time.Time, attrs ...any) {
	elapsed := time.Since(started)
	level := slog.LevelDebug
	if elapsed >= time.Second {
		level = slog.LevelInfo
	}
	fields := []any{"component", "bigquery", "event.name", "bigquery.write.timing", "run_id", s.run, "resource", resource, "phase", phase, "duration_ms", elapsed.Milliseconds()}
	fields = append(fields, attrs...)
	slog.Log(context.Background(), level, "BigQuery write timing", fields...)
}
