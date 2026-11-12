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

	"github.com/galaxy-io/filament"
	icebergcatalog "github.com/galaxy-io/filament/connectors/iceberg/internal/catalog"
	"github.com/galaxy-io/filament/internal/sinkstate"
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
	policies  map[string]filament.WritePolicy
	lifecycle sinkstate.Lifecycle
	active    *stage

	tableLocationRoot  string
	namespace          string
	stageBufLimitBytes int64
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
	s.policies = maps.Clone(run.WritePolicies)
	s.writeModes = map[string]writeMode{}
	s.tables = map[string]*iceTable{}
	s.active = nil
	s.mu.Unlock()
	return nil
}

// ── internals ─────────────────────────────────────────────────────────────────

// promoteStage commits each resource once. A partial failure poisons the session;
// Abort releases remaining buffers without reverting committed tables.
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
	return nil
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
