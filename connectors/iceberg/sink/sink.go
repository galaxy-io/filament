// Package iceberg implements filament.Sink writing Arrow batches as Parquet data
// files into Iceberg tables managed by any catalog backend registered with
// iceberg-go.
package iceberg

import (
	"context"
	"fmt"
	"maps"
	"sync"

	iceberg "github.com/apache/iceberg-go"
	icecatalog "github.com/apache/iceberg-go/catalog"
	icetable "github.com/apache/iceberg-go/table"
	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	icebergcatalog "github.com/galaxy-io/filament/connectors/iceberg/internal/catalog"
)

const (
	defaultStageBufLimitBytes = 256 << 20 // 256 MiB
	defaultNamespace          = "default"
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
	tableLocationRoot  string
	namespace          string
	stageBufLimitBytes int64
	ingestionTypes     map[string]filament.IngestionType
	writeModes         map[string]writeMode
	run                filament.RunID

	cat icecatalog.Catalog

	mu       sync.Mutex
	tables   map[string]*iceTable // populated by EnsureSchema
	stages   map[filament.StageID]*stage
	curStage filament.StageID // the stage Apply targets; "" until first Stage/Apply
}

type iceTable struct {
	tbl        *icetable.Table
	schema     *iceberg.Schema
	primaryKey []string
}

// stage holds per-resource row buffers for one pending commit.
type stage struct {
	id                      filament.StageID
	mu                      sync.Mutex
	buf                     map[string]*recordBuf
	committed               map[string]bool
	includeReplaceResources bool
}

// New returns an unconfigured iceberg sink.
func New() *Sink {
	return &Sink{
		tables:     map[string]*iceTable{},
		writeModes: map[string]writeMode{},
		stages:     map[filament.StageID]*stage{},
	}
}

var (
	_ filament.Sink            = (*Sink)(nil)
	_ filament.LiveValidatable = (*Sink)(nil)
	_ filament.Transactional   = (*Sink)(nil)
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
	s.ingestionTypes = maps.Clone(run.IngestionTypes)
	s.writeModes = map[string]writeMode{}
	s.tables = map[string]*iceTable{}
	s.stages = map[filament.StageID]*stage{}
	s.curStage = ""
	s.mu.Unlock()
	return nil
}

// Stage opens a new staging scope; batches applied under it commit atomically.
func (s *Sink) Stage(_ context.Context) (filament.StageID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cat == nil {
		return "", fmt.Errorf("iceberg sink: Stage called before Open")
	}
	id := filament.StageID(uuid.NewString())
	s.stages[id] = newStage(id)
	s.curStage = id
	return id, nil
}

// Promote drains each resource buffer into the Iceberg table, one transaction
// per resource, then drops the stage after every resource succeeds.
func (s *Sink) Promote(ctx context.Context, id filament.StageID) error {
	s.mu.Lock()
	st := s.stages[id]
	s.mu.Unlock()
	if st == nil {
		return fmt.Errorf("iceberg sink: unknown stage %s", id)
	}
	return s.promoteStage(ctx, st)
}

// Commit promotes the active stage so the direct Open→Apply→Commit lifecycle
// lands data. A stage already flushed via Promote leaves nothing to do.
func (s *Sink) Commit(ctx context.Context) error {
	s.mu.Lock()
	st := s.stages[s.curStage]
	hasReplace := s.hasReplaceResourceLocked()
	if st == nil && hasReplace {
		st = newStage(filament.StageID(uuid.NewString()))
		s.stages[st.id] = st
		s.curStage = st.id
	}
	s.mu.Unlock()
	if st == nil {
		return nil
	}
	if hasReplace {
		st.mu.Lock()
		st.includeReplaceResources = true
		st.mu.Unlock()
	}
	return s.promoteStage(ctx, st)
}

// Abort discards all buffers and removes any temp files.
func (s *Sink) Abort(_ context.Context) error {
	s.mu.Lock()
	stages := s.stages
	s.stages = map[filament.StageID]*stage{}
	s.curStage = ""
	s.mu.Unlock()
	for _, st := range stages {
		st.mu.Lock()
		for _, rb := range st.buf {
			rb.close()
		}
		st.mu.Unlock()
	}
	return nil
}

// ── internals ─────────────────────────────────────────────────────────────────

// promoteStage flushes every resource buffer in a stage and drops it only after
// every resource succeeds. On failure the stage stays open so Commit/Promote can
// be retried; resources that already committed are skipped on retry.
func (s *Sink) promoteStage(ctx context.Context, st *stage) error {
	st.mu.Lock()
	defer st.mu.Unlock()

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
		if err := s.writeBuffer(ctx, it, rb, mode); err != nil {
			return fmt.Errorf("iceberg sink: commit %s: %w", resource, err)
		}
		st.committed[resource] = true
	}
	for _, rb := range st.buf {
		rb.close()
	}
	s.dropStage(st.id)
	return nil
}

// activeStageLocked returns the stage Apply targets, opening one if none exists.
// Caller holds s.mu.
func (s *Sink) activeStageLocked() *stage {
	if st := s.stages[s.curStage]; st != nil {
		return st
	}
	id := filament.StageID(uuid.NewString())
	st := newStage(id)
	s.stages[id] = st
	s.curStage = id
	return st
}

func newStage(id filament.StageID) *stage {
	return &stage{id: id, buf: map[string]*recordBuf{}, committed: map[string]bool{}}
}

func (s *Sink) stageResources(st *stage) []string {
	seen := make(map[string]bool, len(st.buf))
	out := make([]string, 0, len(st.buf))
	for resource := range st.buf {
		seen[resource] = true
		out = append(out, resource)
	}
	if !st.includeReplaceResources {
		return out
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for resource := range s.tables {
		if !seen[resource] && s.writeModes[resource] == writeModeReplace {
			out = append(out, resource)
		}
	}
	return out
}

func (s *Sink) hasReplaceResourceLocked() bool {
	for resource := range s.tables {
		if s.writeModes[resource] == writeModeReplace {
			return true
		}
	}
	return false
}

func (s *Sink) writeModeForResource(resource string) writeMode {
	return writeModeForPolicy(filament.TypeFor(s.ingestionTypes, resource).WritePolicy())
}

func (s *Sink) dropStage(id filament.StageID) {
	s.mu.Lock()
	delete(s.stages, id)
	if s.curStage == id {
		s.curStage = ""
	}
	s.mu.Unlock()
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
