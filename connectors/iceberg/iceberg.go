// Package iceberg implements filament.Sink writing records as Parquet data files into
// Iceberg tables managed by any catalog backend registered with iceberg-go.
package iceberg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	iceberg "github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/catalog"
	icetable "github.com/apache/iceberg-go/table"
	"github.com/google/uuid"

	// Blank imports register each catalog backend's factory in the iceberg-go
	// catalog registry via its init(). catalog.Load then dispatches on the
	// configured "type" (or URI scheme).
	_ "github.com/apache/iceberg-go/catalog/glue"
	_ "github.com/apache/iceberg-go/catalog/hive"
	_ "github.com/apache/iceberg-go/catalog/rest"
	_ "github.com/apache/iceberg-go/catalog/sql"

	// Registers the FileIO backends (s3/s3a/s3n, gs, azure) that read and write
	// the actual data and metadata files.
	_ "github.com/apache/iceberg-go/io/gocloud"

	"github.com/galaxy-io/filament"
)

const defaultStageBufLimitBytes = 256 << 20 // 256 MiB

type writeMode string

const (
	writeModeAuto    writeMode = "auto"
	writeModeAppend  writeMode = "append"
	writeModeReplace writeMode = "replace"
	writeModeUpsert  writeMode = "upsert"
	writeModeDelete  writeMode = "delete"
	writeModeMerge   writeMode = "merge"
)

// Sink writes to Iceberg tables via any catalog backend registered with
// iceberg-go (selected by the "type" property in catalog config).
type Sink struct {
	warehouse          string
	namespace          string
	stageBufLimitBytes int64
	writeMode          writeMode
	run                filament.RunID

	cat catalog.Catalog

	mu       sync.Mutex
	tables   map[string]*iceTable // populated by EnsureSchema
	stages   map[filament.StageID]*stage
	curStage filament.StageID // the stage Write targets; "" until first Stage/Write
}

type iceTable struct {
	tbl        *icetable.Table
	schema     *iceberg.Schema
	record     filament.RecordSchema
	primaryKey []string
}

// stage holds per-resource record buffers for one pending commit.
type stage struct {
	id                  filament.StageID
	mu                  sync.Mutex
	buf                 map[string]*recordBuf
	committed           map[string]bool
	includeAllResources bool
}

// New returns an unconfigured iceberg sink.
func New() *Sink {
	return &Sink{
		tables: map[string]*iceTable{},
		stages: map[filament.StageID]*stage{},
	}
}

var (
	_ filament.Sink          = (*Sink)(nil)
	_ filament.Transactional = (*Sink)(nil)
	_ filament.Schematized   = (*Sink)(nil)
)

// Spec reports the sink's capabilities and configuration surface.
func (s *Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:        "iceberg",
		DisplayName: "Apache Iceberg",
		Version:     "1",
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "warehouse", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "Warehouse root location (shared catalog storage)."},
			{Name: "catalog", Type: filament.FieldObject, Required: true, Scope: filament.ScopeConnection, Help: "Catalog connection props; requires type or uri (plus backend credentials)."},
			{Name: "namespace", Type: filament.FieldString, Required: true, Scope: filament.ScopePipeline, Help: "Destination namespace (database) for this pipeline's tables."},
			{Name: "write_mode", Type: filament.FieldEnum, Enum: []string{"auto", "append", "replace", "upsert", "delete", "merge"}, Default: "auto", Scope: filament.ScopePipeline, Help: "Write behavior; auto picks replace for full loads, append otherwise."},
			{Name: "stage_buffer_limit_mb", Type: filament.FieldInt, Scope: filament.ScopePipeline, Help: "Staging buffer flush threshold in MiB."},
		}},
		Capabilities: filament.SinkCapabilities{
			Transactional: true,
			Schematized:   true,
			WritePolicies: filament.WriteCapabilities(
				filament.IngestionSnapshotReplace,
				filament.IngestionAppend,
				filament.IngestionSnapshotUpsert,
				filament.IngestionUpsert,
				filament.IngestionDelete,
				filament.IngestionCDC,
			),
		},
	}
}

// Name identifies this sink implementation.
func (s *Sink) Name() string { return "iceberg" }

// Open connects to the catalog and prepares per-resource tables for the run.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	cfg := filament.NewConfig(run.Sink.Config)
	s.warehouse = cfg.String("warehouse")
	if s.warehouse == "" {
		return fmt.Errorf("iceberg sink: warehouse required")
	}
	s.namespace = cfg.String("namespace")
	if s.namespace == "" {
		return fmt.Errorf("iceberg sink: namespace required")
	}
	s.run = run.Run

	s.stageBufLimitBytes = defaultStageBufLimitBytes
	if mb := cfg.Int("stage_buffer_limit_mb"); mb > 0 {
		s.stageBufLimitBytes = int64(mb) << 20
	}
	mode, err := resolveWriteMode(cfg.String("write_mode"), run.Mode)
	if err != nil {
		return err
	}

	// Build catalog properties from the "catalog" sub-config and hand them to
	// catalog.Load, which dispatches on props["type"] (or the uri scheme) to the
	// matching registered backend. The warehouse is shared with the data writer.
	props := iceberg.Properties{"warehouse": s.warehouse}
	for k, v := range cfg.Sub("catalog").Raw() {
		props[k] = fmt.Sprint(v)
	}
	if props["type"] == "" && props["uri"] == "" {
		return fmt.Errorf("iceberg sink: catalog.type or catalog.uri required")
	}
	cat, err := catalog.Load(ctx, "iceberg", props)
	if err != nil {
		return fmt.Errorf("iceberg sink: open catalog: %w", err)
	}

	s.mu.Lock()
	s.cat = cat
	s.writeMode = mode
	s.tables = map[string]*iceTable{}
	s.stages = map[filament.StageID]*stage{}
	s.curStage = ""
	s.mu.Unlock()
	return nil
}

// EnsureSchema creates the Iceberg table if absent, or evolves it by adding any
// columns not yet present.
func (s *Sink) EnsureSchema(ctx context.Context, resource string, schema filament.RecordSchema) error {
	s.mu.Lock()
	cat := s.cat
	s.mu.Unlock()
	if cat == nil {
		return fmt.Errorf("iceberg sink: EnsureSchema called before Open")
	}

	// Create the namespace first: a strict REST catalog rejects CreateTable with
	// NoSuchNamespace otherwise. Idempotent — an existing namespace is fine.
	nsIdent := catalog.ToIdentifier(splitNamespace(s.namespace)...)
	if err := cat.CreateNamespace(ctx, nsIdent, nil); err != nil &&
		!errors.Is(err, catalog.ErrNamespaceAlreadyExists) {
		return fmt.Errorf("iceberg sink: create namespace %s: %w", s.namespace, err)
	}

	iceSchema := buildIcebergSchema(schema)
	ident := s.tableIdent(resource)
	location := joinURI(s.warehouse, namespacePath(s.namespace), resource)

	tbl, err := cat.CreateTable(ctx, ident, iceSchema,
		catalog.WithLocation(location),
		catalog.WithProperties(iceberg.Properties{"write.format.default": "parquet"}),
	)
	if errors.Is(err, catalog.ErrTableAlreadyExists) {
		tbl, err = cat.LoadTable(ctx, ident)
		if err != nil {
			return fmt.Errorf("iceberg sink: load table %s: %w", resource, err)
		}
		if err := evolveSchema(ctx, tbl, schema); err != nil {
			return fmt.Errorf("iceberg sink: evolve schema %s: %w", resource, err)
		}
		tbl, err = cat.LoadTable(ctx, ident)
		if err != nil {
			return fmt.Errorf("iceberg sink: reload table %s: %w", resource, err)
		}
	} else if err != nil {
		return fmt.Errorf("iceberg sink: create table %s: %w", resource, err)
	}

	s.mu.Lock()
	s.tables[resource] = &iceTable{
		tbl:        tbl,
		schema:     tbl.Schema(),
		record:     schema,
		primaryKey: append([]string(nil), schema.PrimaryKey...),
	}
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

// Write buffers each record's JSON payload into the active stage, auto-opening
// one if Stage was never called (the direct filament.Sink lifecycle). Spills to a
// temp file once the per-resource byte total exceeds stageBufLimitBytes.
func (s *Sink) Write(_ context.Context, b filament.Batch) (filament.WriteReceipt, error) {
	s.mu.Lock()
	it := s.tables[b.Resource]
	if it == nil {
		s.mu.Unlock()
		return filament.WriteReceipt{}, fmt.Errorf("iceberg sink: no schema ensured for resource %q", b.Resource)
	}
	st := s.activeStageLocked()
	s.mu.Unlock()

	st.mu.Lock()
	rb := st.buf[b.Resource]
	if rb == nil {
		rb = newRecordBuf(s.stageBufLimitBytes)
		st.buf[b.Resource] = rb
	}
	var nbytes int64
	for i := range b.Records {
		if err := rb.appendRecord(b.Records[i].Data, b.Records[i].Op); err != nil {
			st.mu.Unlock()
			return filament.WriteReceipt{}, fmt.Errorf("iceberg sink: buffer %s: %w", b.Resource, err)
		}
		nbytes += int64(len(b.Records[i].Data))
	}
	st.mu.Unlock()

	crc, _ := filament.CRC32C(b.Records)
	return filament.WriteReceipt{
		URI:      joinURI(s.warehouse, namespacePath(s.namespace), b.Resource),
		Bytes:    nbytes,
		Rows:     len(b.Records),
		WriteCRC: crc,
	}, nil
}

// Apply validates the batch against the run's write policy and buffers it
// for the resource's table.
func (s *Sink) Apply(_ context.Context, b filament.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.mu.Lock()
	it := s.tables[b.Resource]
	if it == nil {
		s.mu.Unlock()
		return filament.WriteReceipt{}, fmt.Errorf("iceberg sink: no schema ensured for resource %q", b.Resource)
	}
	policy := opts.Policy
	if len(policy.Keys) == 0 {
		policy.Keys = append([]string(nil), it.primaryKey...)
	}
	if policy.Capability.RequiresPK && len(policy.Keys) == 0 {
		s.mu.Unlock()
		return filament.WriteReceipt{}, fmt.Errorf("iceberg sink: write policy %q requires primary key for resource %q", policy.Capability.Mode, b.Resource)
	}
	st := s.activeStageLocked()
	s.mu.Unlock()

	st.mu.Lock()
	rb := st.buf[b.Resource]
	if rb == nil {
		rb = newRecordBuf(s.stageBufLimitBytes)
		st.buf[b.Resource] = rb
	}
	if err := rb.setPolicy(policy); err != nil {
		st.mu.Unlock()
		return filament.WriteReceipt{}, fmt.Errorf("iceberg sink: buffer %s: %w", b.Resource, err)
	}
	var nbytes int64
	if err := policy.ValidateRecords(b.Resource, b.Records); err != nil {
		st.mu.Unlock()
		return filament.WriteReceipt{}, fmt.Errorf("iceberg sink: %w", err)
	}
	for i := range b.Records {
		if err := rb.appendRecord(b.Records[i].Data, b.Records[i].Op); err != nil {
			st.mu.Unlock()
			return filament.WriteReceipt{}, fmt.Errorf("iceberg sink: buffer %s: %w", b.Resource, err)
		}
		nbytes += int64(len(b.Records[i].Data))
	}
	st.mu.Unlock()

	crc, _ := filament.CRC32C(b.Records)
	return filament.WriteReceipt{
		URI:      joinURI(s.warehouse, namespacePath(s.namespace), b.Resource),
		Bytes:    nbytes,
		Rows:     len(b.Records),
		WriteCRC: crc,
	}, nil
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

// Commit promotes the active stage so the direct Open→Write→Commit lifecycle
// lands data. A stage already flushed via Promote leaves nothing to do.
func (s *Sink) Commit(ctx context.Context) error {
	s.mu.Lock()
	st := s.stages[s.curStage]
	if st == nil && s.writeMode == writeModeReplace && len(s.tables) > 0 {
		st = newStage(filament.StageID(uuid.NewString()))
		s.stages[st.id] = st
		s.curStage = st.id
	}
	s.mu.Unlock()
	if st == nil {
		return nil
	}
	if s.writeMode == writeModeReplace {
		st.mu.Lock()
		st.includeAllResources = true
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
		mode := s.writeMode
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

// activeStageLocked returns the stage Write targets, opening one if none exists.
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
	if !st.includeAllResources {
		return out
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for resource := range s.tables {
		if !seen[resource] {
			out = append(out, resource)
		}
	}
	return out
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
	return catalog.ToIdentifier(append(parts, resource)...)
}

func resolveWriteMode(configured string, runMode filament.ReplicationMode) (writeMode, error) {
	mode := writeMode(strings.ToLower(strings.TrimSpace(configured)))
	if mode == "" {
		mode = writeModeAuto
	}
	switch mode {
	case writeModeAuto:
		if runMode == filament.ModeFull {
			return writeModeReplace, nil
		}
		return writeModeAppend, nil
	case writeModeAppend, writeModeReplace, writeModeUpsert, writeModeDelete, writeModeMerge:
		return mode, nil
	default:
		return "", fmt.Errorf("iceberg sink: invalid write_mode %q (want auto, append, replace, upsert, delete, or merge)", configured)
	}
}
