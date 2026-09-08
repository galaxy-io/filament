// Package sink implements the filament.Sink interface for Meilisearch.
// It streams Arrow batches as NDJSON directly into Meilisearch indexes,
// leveraging Meilisearch's internal task queue, auto-batching, and optional
// gzip compression for high-performance ingestion.
package sink

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/internal/ndjson"
	"github.com/galaxy-io/filament/rowmodel"
)

type indexMeta struct {
	UID           string
	PrimaryKey    string
	PrimaryKeyIdx int
}

// Sink writes Arrow record batches into Meilisearch indexes.
type Sink struct {
	mu      sync.Mutex
	cfg     Config
	client  *Client
	run     filament.RunID
	indexes map[string]*indexMeta
	enc     map[*arrow.Schema]*ndjson.Encoder
	buf     []byte
	tasks   []int64
	written atomic.Int64
	aborted bool
}

// New returns an unconfigured Meilisearch sink.
func New() *Sink {
	return &Sink{
		indexes: make(map[string]*indexMeta),
		enc:     make(map[*arrow.Schema]*ndjson.Encoder),
	}
}

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.Schematized       = (*Sink)(nil)
)

// Spec describes the sink's capabilities, write policies, and configuration schema.
func (s *Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         "meilisearch",
		DisplayName:  "Meilisearch",
		Description:  "Fast, open-source search engine with typo-tolerant full-text search.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-meilisearch-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-meilisearch-light.svg",
		Version:      "1",
		Config:       ConfigSchema(),
		Capabilities: filament.SinkCapabilities{
			Schematized:        true,
			Upsertable:         true,
			EncodedIntegrity:   true,
			PreferredBatchRows: defaultBatchSize,
			WritePolicies: commitDurableCapabilities(
				filament.IngestionFullReplace,
				filament.IngestionFullAppend,
				filament.IngestionFullUpsert,
				filament.IngestionIncrementalUpsert,
				filament.IngestionCDCMerge,
				filament.IngestionCDCAppend,
			),
		},
		SchemaField: "index_prefix",
	}
}

func commitDurableCapabilities(types ...filament.IngestionType) []filament.WritePolicyCapability {
	capabilities := filament.WriteCapabilities(types...)
	for i := range capabilities {
		capabilities[i].Durability = filament.DurabilityAfterCommit
		capabilities[i].Atomicity = filament.AtomicityResource
	}
	return capabilities
}

// Name identifies this sink implementation.
func (s *Sink) Name() string { return "meilisearch" }

// Validate checks connection configuration syntax without making network requests.
func (s *Sink) Validate(cfg filament.Config) error {
	_, err := ParseConfig(cfg)
	return err
}

// TestConnection verifies connectivity to the Meilisearch server by calling /health.
func (s *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	parsed, err := ParseConfig(cfg)
	if err != nil {
		return err
	}
	client := NewClient(parsed.URL, parsed.APIKey, false)
	return client.Health(ctx)
}

// Open prepares the sink for a run.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	parsed, err := ParseConfig(filament.NewConfig(run.Sink.Config))
	if err != nil {
		return fmt.Errorf("meilisearch sink: open: %w", err)
	}

	s.cfg = parsed
	s.client = NewClient(parsed.URL, parsed.APIKey, parsed.Gzip)
	s.run = run.Run
	s.indexes = make(map[string]*indexMeta)
	s.enc = make(map[*arrow.Schema]*ndjson.Encoder)
	s.tasks = nil
	s.aborted = false

	return nil
}

// EnsureSchema ensures destination index exists in Meilisearch and prepares primary key metadata.
func (s *Sink) EnsureSchema(ctx context.Context, resource string, schema rowmodel.Schema) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client == nil {
		return fmt.Errorf("meilisearch sink: ensure schema before open")
	}

	indexUID := s.cfg.Index
	if indexUID == "" {
		indexUID = sanitizeIndexUID(s.cfg.IndexPrefix, resource)
	}

	primaryKey := s.cfg.PrimaryKey
	if primaryKey == "" {
		if len(schema.PrimaryKey) == 1 {
			primaryKey = schema.PrimaryKey[0]
		} else {
			// Check if any field is named "id" or "ID"
			for _, f := range schema.Fields {
				if strings.EqualFold(f.Name, "id") {
					primaryKey = f.Name
					break
				}
			}
			// If still empty, check for field ending in _id or Id
			if primaryKey == "" {
				for _, f := range schema.Fields {
					if strings.HasSuffix(strings.ToLower(f.Name), "_id") || strings.HasSuffix(f.Name, "Id") {
						primaryKey = f.Name
						break
					}
				}
			}
			// Fallback to first field
			if primaryKey == "" && len(schema.Fields) > 0 {
				primaryKey = schema.Fields[0].Name
			}
		}
	}

	pkIdx := -1
	for i, f := range schema.Fields {
		if f.Name == primaryKey {
			pkIdx = i
			break
		}
	}

	// Check if index already exists
	existing, err := s.client.GetIndex(ctx, indexUID)
	if err != nil && !errorsIs(err, ErrIndexNotFound) {
		return fmt.Errorf("meilisearch sink: check index %q: %w", indexUID, err)
	}

	if existing == nil {
		// Create index with resolved primary key
		task, err := s.client.CreateIndex(ctx, indexUID, primaryKey)
		if err != nil {
			return fmt.Errorf("meilisearch sink: create index %q: %w", indexUID, err)
		}
		if task != nil && task.TaskUID > 0 {
			s.tasks = append(s.tasks, task.TaskUID)
		}
	} else if existing.PrimaryKey != "" {
		primaryKey = existing.PrimaryKey
		// Re-resolve pkIdx for existing index primary key
		for i, f := range schema.Fields {
			if f.Name == primaryKey {
				pkIdx = i
				break
			}
		}
	}

	s.indexes[resource] = &indexMeta{
		UID:           indexUID,
		PrimaryKey:    primaryKey,
		PrimaryKeyIdx: pkIdx,
	}

	return nil
}

// Apply encodes an Arrow batch into NDJSON and sends it to Meilisearch.
func (s *Sink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if s.client == nil {
		return filament.WriteReceipt{}, fmt.Errorf("meilisearch sink: write before open")
	}

	s.mu.Lock()
	meta := s.indexes[b.Resource]
	if meta == nil {
		// Auto-derive index if EnsureSchema was not invoked
		uid := s.cfg.Index
		if uid == "" {
			uid = sanitizeIndexUID(s.cfg.IndexPrefix, b.Resource)
		}
		meta = &indexMeta{
			UID:           uid,
			PrimaryKey:    s.cfg.PrimaryKey,
			PrimaryKeyIdx: -1,
		}
		s.indexes[b.Resource] = meta
	}
	s.mu.Unlock()

	switch opts.Policy.Capability.Mode {
	case filament.WriteReplace:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("meilisearch sink: %w", err)
		}
		return s.writeBatch(ctx, meta, b, false)

	case filament.WriteAppend:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("meilisearch sink: %w", err)
		}
		return s.writeBatch(ctx, meta, b, false)

	case filament.WriteUpsert:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("meilisearch sink: %w", err)
		}
		return s.writeBatch(ctx, meta, b, false)

	case filament.WriteMerge:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("meilisearch sink: %w", err)
		}
		return s.writeMerge(ctx, meta, b)

	default:
		return filament.WriteReceipt{}, fmt.Errorf("meilisearch sink: write policy %q is not implemented", opts.Policy.Capability.Mode)
	}
}

func (s *Sink) writeBatch(ctx context.Context, meta *indexMeta, b *arrowbatch.Batch, isUpdate bool) (filament.WriteReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows := b.Rows()
	enc := s.enc[rows.Schema()]
	if enc == nil {
		enc = ndjson.NewEncoder(rows.Schema())
		s.enc[rows.Schema()] = enc
	}

	buf, encodedCRC, err := enc.EncodeBatch(s.buf[:0], rows)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("meilisearch: serialize %s: %w", b.Resource, err)
	}
	s.buf = buf

	arrowCRC := b.IntegrityCRC()

	task, err := s.client.AddDocumentsNDJSON(ctx, meta.UID, meta.PrimaryKey, buf, isUpdate)
	if err != nil {
		return filament.WriteReceipt{}, err
	}
	if task != nil && task.TaskUID > 0 {
		s.tasks = append(s.tasks, task.TaskUID)
	}

	nbytes := int64(len(buf))
	s.written.Add(int64(b.NumRows()))

	return filament.WriteReceipt{
		URI:        fmt.Sprintf("meilisearch://%s/%s", s.cfg.URL, meta.UID),
		Bytes:      nbytes,
		Rows:       b.NumRows(),
		WriteCRC:   arrowCRC,
		EncodedCRC: &encodedCRC,
	}, nil
}

func (s *Sink) writeMerge(ctx context.Context, meta *indexMeta, b *arrowbatch.Batch) (filament.WriteReceipt, error) {
	// Separate deletes from inserts/updates
	var deleteIDs []string
	rows := b.Rows()

	if meta.PrimaryKeyIdx >= 0 && meta.PrimaryKeyIdx < int(rows.NumCols()) {
		pkCol := rows.Column(meta.PrimaryKeyIdx)
		for i := range b.NumRows() {
			if b.Op(i) == filament.OpDelete {
				if !pkCol.IsNull(i) {
					deleteIDs = append(deleteIDs, FormatDocumentID(pkCol.ValueStr(i)))
				}
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// If there are deletes, send batch delete
	if len(deleteIDs) > 0 {
		task, err := s.client.DeleteDocumentsBatch(ctx, meta.UID, deleteIDs)
		if err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("meilisearch: cdc delete: %w", err)
		}
		if task != nil && task.TaskUID > 0 {
			s.tasks = append(s.tasks, task.TaskUID)
		}
	}

	// Encode all rows
	enc := s.enc[rows.Schema()]
	if enc == nil {
		enc = ndjson.NewEncoder(rows.Schema())
		s.enc[rows.Schema()] = enc
	}

	buf, encodedCRC, err := enc.EncodeBatch(s.buf[:0], rows)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("meilisearch: serialize %s: %w", b.Resource, err)
	}
	s.buf = buf

	arrowCRC := b.IntegrityCRC()

	// Send documents
	task, err := s.client.AddDocumentsNDJSON(ctx, meta.UID, meta.PrimaryKey, buf, false)
	if err != nil {
		return filament.WriteReceipt{}, err
	}
	if task != nil && task.TaskUID > 0 {
		s.tasks = append(s.tasks, task.TaskUID)
	}

	nbytes := int64(len(buf))
	s.written.Add(int64(b.NumRows()))

	return filament.WriteReceipt{
		URI:        fmt.Sprintf("meilisearch://%s/%s", s.cfg.URL, meta.UID),
		Bytes:      nbytes,
		Rows:       b.NumRows(),
		WriteCRC:   arrowCRC,
		EncodedCRC: &encodedCRC,
	}, nil
}

// Commit waits for all enqueued Meilisearch indexing tasks to finish if WaitForTasks is enabled.
func (s *Sink) Commit(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client == nil || s.aborted {
		return nil
	}

	if s.cfg.WaitForTasks && len(s.tasks) > 0 {
		if err := s.client.WaitForTasks(ctx, s.tasks, s.cfg.TaskTimeout); err != nil {
			return fmt.Errorf("meilisearch sink commit: %w", err)
		}
	}

	s.tasks = nil
	return nil
}

// Abort marks the run aborted and discards tracked tasks.
func (s *Sink) Abort(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.aborted = true
	s.tasks = nil
	return nil
}

func sanitizeIndexUID(prefix, resource string) string {
	raw := resource
	if prefix != "" {
		if !strings.HasSuffix(prefix, "_") && !strings.HasSuffix(prefix, "-") &&
			!strings.HasPrefix(resource, "_") && !strings.HasPrefix(resource, "-") {
			raw = prefix + "_" + resource
		} else {
			raw = prefix + resource
		}
	}
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	s := strings.Trim(b.String(), "_-")
	if s == "" {
		return "default"
	}
	return s
}

func errorsIs(err, target error) bool {
	return err == target || (err != nil && target != nil && err.Error() == target.Error())
}
