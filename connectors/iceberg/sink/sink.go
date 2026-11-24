// Package iceberg implements filament.Sink writing Arrow batches as Parquet data
// files into Iceberg tables managed by any catalog backend registered with
// iceberg-go.
package iceberg

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	iceberg "github.com/apache/iceberg-go"
	icecatalog "github.com/apache/iceberg-go/catalog"
	icetable "github.com/apache/iceberg-go/table"

	"github.com/galaxy-io/filament"
	icebergcatalog "github.com/galaxy-io/filament/connectors/iceberg/internal/catalog"
	"github.com/galaxy-io/filament/internal/sinkstate"
)

const (
	defaultStageBufLimitBytes = 256 << 20 // 256 MiB
	defaultCommitInterval     = time.Minute
	defaultNamespace          = "default"
	writeMergeModeKey         = "write.merge.mode"
	defaultCommitConcurrency  = 8
	maxCommitConcurrency      = 64
)

type writeMode string

const (
	writeModeAppend  writeMode = "append"
	writeModeReplace writeMode = "replace"
	writeModeUpsert  writeMode = "upsert"
	writeModeDelete  writeMode = "delete"
	writeModeMerge   writeMode = "merge"
)

// Sink writes to Iceberg tables via any catalog backend registered with
// iceberg-go (selected by the "type" property in catalog config).
type Sink struct {
	policies  map[string]filament.WritePolicy
	lifecycle sinkstate.Lifecycle
	active    *stage
	// uncertain is a failed commit that may still have reached the catalog, or
	// that failed after another table committed. Replaying either can
	// duplicate rows, so the session cannot close clean.
	uncertain error

	tableLocationRoot  string
	tableUpdateMode    string
	namespace          string
	stageBufLimitBytes int64
	commitConcurrency  int
	ingestionTypes     map[string]filament.IngestionType
	writeModes         map[string]writeMode
	run                filament.RunID

	cat icecatalog.Catalog

	mu     sync.Mutex
	tables map[string]*iceTable // populated by EnsureSchema
}

type iceTable struct {
	tbl        *icetable.Table
	schema     *iceberg.Schema
	primaryKey []string
}

// stage holds per-resource row buffers for one pending commit.
type stage struct {
	mu        sync.Mutex
	buf       map[string]*recordBuf
	committed map[string]bool
}

// New returns an unconfigured iceberg sink.
func New() *Sink {
	return &Sink{
		tables:     map[string]*iceTable{},
		writeModes: map[string]writeMode{},
	}
}

var (
	_ filament.Sink            = (*Sink)(nil)
	_ filament.LiveValidatable = (*Sink)(nil)
	_ filament.Schematized     = (*Sink)(nil)
)

// Name identifies this sink implementation.
func (s *Sink) Name() string { return "iceberg" }

// TestConnection loads the configured catalog and performs a read-only
// namespace listing. This exercises catalog authentication without creating
// a namespace or table.
func (s *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	setup, err := icebergcatalog.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("iceberg sink: %w", err)
	}
	if err := icebergcatalog.Test(ctx, setup); err != nil {
		return fmt.Errorf("iceberg sink: %w", err)
	}
	return nil
}

// Open connects to the catalog and prepares per-resource tables for the run.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	if err := s.open(ctx, run); err != nil {
		return err
	}
	s.lifecycle.Open()
	return nil
}

func (s *Sink) open(ctx context.Context, run filament.RunSpec) error {
	cfg := filament.NewConfig(run.Sink.Config)
	s.namespace = cfg.String("namespace")
	if s.namespace == "" {
		s.namespace = defaultNamespace
	}
	s.run = run.Run

	s.stageBufLimitBytes = defaultStageBufLimitBytes
	if mb := cfg.Int("stage_buffer_limit_mb"); mb > 0 {
		s.stageBufLimitBytes = int64(mb) << 20
	}
	s.commitConcurrency = defaultCommitConcurrency
	if cfg.Has("commit_concurrency") {
		n := cfg.Int("commit_concurrency")
		if n < 1 || n > maxCommitConcurrency {
			return fmt.Errorf("iceberg sink: commit_concurrency must be between 1 and %d", maxCommitConcurrency)
		}
		s.commitConcurrency = n
	}
	setup, err := icebergcatalog.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("iceberg sink: %w", err)
	}
	cat, err := icebergcatalog.Open(ctx, "iceberg", setup)
	if err != nil {
		return fmt.Errorf("iceberg sink: open catalog: %w", err)
	}

	s.mu.Lock()
	s.cat = cat
	s.tableLocationRoot = setup.TableLocationRoot
	s.tableUpdateMode = setup.TableUpdateMode
	s.ingestionTypes = maps.Clone(run.IngestionTypes)
	s.policies = maps.Clone(run.WritePolicies)
	s.writeModes = map[string]writeMode{}
	s.tables = map[string]*iceTable{}
	s.active = nil
	s.mu.Unlock()
	return nil
}

// ── internals ─────────────────────────────────────────────────────────────────

// commitJob is one resource's pending table commit.
type commitJob struct {
	resource string
	it       *iceTable
	rb       *recordBuf
	mode     writeMode
}

// promoteStage commits each resource once, each in its own table transaction.
// Every resource is validated before any commits. Append and replace commits
// are cheap now that their files are written during the run, so they run
// commitConcurrency at a time; upsert, delete, and merge rewrite existing data
// files and run one at a time so their memory does not stack. A failure starts
// no further commits, lets those in flight finish, and poisons the session;
// Abort releases the remaining buffers without reverting committed tables.
func (s *Sink) promoteStage(ctx context.Context, st *stage) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	var streamed, rewrites []commitJob
	for _, resource := range s.stageResources(st) {
		if st.committed[resource] {
			continue
		}
		s.mu.Lock()
		it := s.tables[resource]
		mode := s.writeModes[resource]
		if mode == "" {
			mode = writeModeReplace
		}
		limit := s.stageBufLimitBytes
		s.mu.Unlock()
		if it == nil {
			return fmt.Errorf("iceberg sink: promote %s: no schema", resource)
		}
		rb := st.buf[resource]
		if rb == nil {
			if mode != writeModeReplace {
				continue
			}
			rb = newRecordBuf(limit)
			st.buf[resource] = rb
		}
		mode = rb.writeMode(mode)
		if mode == writeModeAppend && rb.count == 0 {
			continue
		}
		if err := rb.validate(mode); err != nil {
			return fmt.Errorf("iceberg sink: validate %s: %w", resource, err)
		}
		job := commitJob{resource: resource, it: it, rb: rb, mode: mode}
		if mode == writeModeAppend || mode == writeModeReplace {
			streamed = append(streamed, job)
		} else {
			rewrites = append(rewrites, job)
		}
	}
	if err := s.commitJobs(ctx, st, streamed, s.commitConcurrency); err != nil {
		if s.uncertain == nil && len(st.committed) > 0 {
			s.uncertain = err
		}
		return err
	}
	if err := s.commitJobs(ctx, st, rewrites, 1); err != nil {
		if s.uncertain == nil && len(st.committed) > 0 {
			s.uncertain = err
		}
		return err
	}
	// Every resource committed, so close deletes no data files; it only frees
	// buffers and temp files, and a failure there does not undo the commit.
	for _, rb := range st.buf {
		_ = rb.close(ctx)
	}
	return nil
}

// commitJobs commits jobs with up to workers in flight. After the first
// failure it starts no more, waits for those in flight, and returns the
// failure. A commit is never cancelled midway: its outcome would be unknown.
// Any job whose commit may still have landed marks the session uncertain.
func (s *Sink) commitJobs(ctx context.Context, st *stage, jobs []commitJob, workers int) error {
	var (
		mu    sync.Mutex
		first error
		wg    sync.WaitGroup
	)
	failed := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return first != nil
	}
	slots := make(chan struct{}, max(1, workers))
	for _, job := range jobs {
		slots <- struct{}{}
		if failed() {
			<-slots
			break
		}
		wg.Go(func() {
			defer func() { <-slots }()
			err := s.writeBuffer(ctx, job.it, job.rb, job.mode)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				err = fmt.Errorf("iceberg sink: commit %s: %w", job.resource, err)
				if first == nil {
					first = err
				}
				// A commit that may have landed counts even behind an earlier failure.
				if isUncertainCommit(err) {
					s.uncertain = err
				}
				return
			}
			st.committed[job.resource] = true
		})
	}
	wg.Wait()
	return first
}

func (s *Sink) stageResources(st *stage) []string {
	seen := make(map[string]bool, len(st.buf))
	out := make([]string, 0, len(st.buf))
	for resource := range st.buf {
		seen[resource] = true
		out = append(out, resource)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for resource := range s.tables {
		if !seen[resource] && s.writeModes[resource] == writeModeReplace {
			out = append(out, resource)
		}
	}
	// A fixed order makes which tables a failed commit left untouched
	// reproducible.
	slices.Sort(out)
	return out
}

func (s *Sink) tableIdent(resource string) icetable.Identifier {
	parts := splitNamespace(s.namespace)
	return icecatalog.ToIdentifier(append(parts, tableName(resource))...)
}

func writeModeForPolicy(policy filament.WritePolicy) writeMode {
	switch policy.Capability.Mode {
	case filament.WriteAppend:
		return writeModeAppend
	case filament.WriteUpsert:
		return writeModeUpsert
	case filament.WriteDelete:
		return writeModeDelete
	case filament.WriteMerge:
		return writeModeMerge
	default:
		return writeModeReplace
	}
}

func (s *Sink) writeModeForResource(resource string) writeMode {
	policy, ok := s.policies[resource]
	if !ok {
		policy, ok = s.policies[""]
	}
	if !ok {
		policy = filament.TypeFor(s.ingestionTypes, resource).WritePolicy()
	}
	return writeModeForPolicy(policy)
}
