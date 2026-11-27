package sink

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/internal/embedding"
	"github.com/galaxy-io/filament/rowmodel"
)

// Sink writes Arrow record batches into Qdrant vector collections.
type Sink struct {
	mu       sync.Mutex
	cfg      Config
	client   *Client
	run      filament.RunID
	policies map[string]filament.WritePolicy
	cleared  map[string]struct{}
	written  atomic.Int64
	aborted  bool
}

// New returns an unconfigured Qdrant sink.
func New() *Sink {
	return &Sink{
		policies: make(map[string]filament.WritePolicy),
		cleared:  make(map[string]struct{}),
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
		Name:         "qdrant",
		DisplayName:  "Qdrant",
		Description:  "Vector similarity search engine with extended payload filtering.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-qdrant-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-qdrant-light.svg",
		Version:      "1",
		Config:       ConfigSchema(),
		Capabilities: filament.SinkCapabilities{
			Schematized:        true,
			PreferredBatchRows: defaultBatchSize,
			WritePolicies: filament.WriteCapabilities(
				filament.IngestionFullReplace,
				filament.IngestionFullAppend,
				filament.IngestionFullUpsert,
				filament.IngestionIncrementalUpsert,
				filament.IngestionCDCMerge,
			),
		},
		SchemaField: "collection",
	}
}

// Name identifies this sink implementation.
func (s *Sink) Name() string { return "qdrant" }

// Validate checks connection configuration syntax without making network requests.
func (s *Sink) Validate(cfg filament.Config) error {
	_, err := ParseConfig(cfg)
	return err
}

// TestConnection verifies connectivity to the Qdrant cluster.
func (s *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	parsed, err := ParseConfig(cfg)
	if err != nil {
		return err
	}
	client := NewClient(parsed)
	return client.Health(ctx)
}

// Open prepares the sink for an ingestion run.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	parsed, err := ParseConfig(filament.NewConfig(run.Sink.Config))
	if err != nil {
		return fmt.Errorf("qdrant sink: open: %w", err)
	}

	s.cfg = parsed
	s.client = NewClient(parsed)
	s.run = run.Run
	s.policies = run.WritePolicies
	s.cleared = make(map[string]struct{})
	s.aborted = false

	return nil
}

// EnsureSchema prepares collection settings before records arrive.
func (s *Sink) EnsureSchema(ctx context.Context, resource string, schema rowmodel.Schema) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client == nil {
		return fmt.Errorf("qdrant sink: ensure schema before open")
	}

	return nil
}

// Apply converts an Arrow batch into vector operations (upsert / delete) and writes to Qdrant in row order.
func (s *Sink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.mu.Lock()
	if s.client == nil {
		s.mu.Unlock()
		return filament.WriteReceipt{}, fmt.Errorf("qdrant sink: write before open")
	}

	if s.aborted {
		s.mu.Unlock()
		return filament.WriteReceipt{}, fmt.Errorf("qdrant sink: cannot apply batch to aborted run")
	}

	collection := s.cfg.Collection
	if collection == "" {
		collection = b.Resource
	}

	mode := opts.Policy.Capability.Mode
	if mode == filament.WriteReplace {
		if _, done := s.cleared[collection]; !done {
			if err := s.client.DeleteAllDocuments(ctx, collection); err != nil {
				s.mu.Unlock()
				return filament.WriteReceipt{}, fmt.Errorf("qdrant sink: clear collection %q for replace: %w", collection, err)
			}
			s.cleared[collection] = struct{}{}
		}
	}
	s.mu.Unlock()

	numRows := b.NumRows()
	if numRows == 0 {
		return filament.WriteReceipt{}, nil
	}

	opStrings := make([]string, numRows)
	for i := 0; i < numRows; i++ {
		op := b.Op(i)
		opStr, err := embedding.OpString(op)
		if err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("qdrant sink: row %d has invalid operation: %w", i, err)
		}
		opStrings[i] = opStr
	}

	rows := b.Rows()
	if rows == nil {
		return filament.WriteReceipt{}, nil
	}

	schema := rows.Schema()
	pkColIdx := embedding.FindColumnIndex(schema, s.cfg.PrimaryKey)
	if pkColIdx < 0 {
		return filament.WriteReceipt{}, fmt.Errorf("qdrant sink: primary key column %q not found in resource %q schema", s.cfg.PrimaryKey, b.Resource)
	}

	// Process operations in contiguous runs to preserve strict CDC row order
	var pendingUpserts []embedding.VectorDoc
	var pendingDeletes []string

	flush := func() error {
		if len(pendingUpserts) > 0 {
			if err := s.client.BatchUpsert(ctx, collection, pendingUpserts); err != nil {
				return fmt.Errorf("qdrant sink: batch upsert: %w", err)
			}
			pendingUpserts = pendingUpserts[:0]
		}
		if len(pendingDeletes) > 0 {
			if err := s.client.BatchDelete(ctx, collection, pendingDeletes); err != nil {
				return fmt.Errorf("qdrant sink: batch delete: %w", err)
			}
			pendingDeletes = pendingDeletes[:0]
		}
		return nil
	}

	for i := 0; i < numRows; i++ {
		docID, err := embedding.ExtractDocID(rows, i, pkColIdx)
		if err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("qdrant sink: extract primary key for row %d: %w", i, err)
		}

		opStr := opStrings[i]

		if opStr == "delete" {
			if len(pendingUpserts) > 0 {
				if err := flush(); err != nil {
					return filament.WriteReceipt{}, err
				}
			}
			pendingDeletes = append(pendingDeletes, docID)
		} else {
			if len(pendingDeletes) > 0 {
				if err := flush(); err != nil {
					return filament.WriteReceipt{}, err
				}
			}

			docText := embedding.ExtractDocumentText(rows, i, s.cfg.TextFields)
			vector, err := embedding.ExtractEmbeddingVector(rows, i, s.cfg.EmbeddingField)
			if err != nil {
				return filament.WriteReceipt{}, fmt.Errorf("qdrant sink: extract embedding vector for row %d: %w", i, err)
			}

			meta := embedding.ExtractMetadataMap(rows, i, s.cfg.EmbeddingField, b.Resource, b.Seq)

			doc := embedding.VectorDoc{
				ID:        docID,
				Vector:    vector,
				Document:  docText,
				Operation: opStr,
				Metadata:  meta,
			}
			pendingUpserts = append(pendingUpserts, doc)
		}
	}

	if err := flush(); err != nil {
		return filament.WriteReceipt{}, err
	}

	s.written.Add(int64(numRows))
	return filament.WriteReceipt{
		URI:   fmt.Sprintf("qdrant://%s/%s", s.cfg.URL, collection),
		Rows:  numRows,
		Bytes: 0,
	}, nil
}

// Commit finalizes active batch transactions.
func (s *Sink) Commit(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.aborted {
		return fmt.Errorf("qdrant sink: cannot commit aborted run")
	}

	return nil
}

// Abort rolls back pending uncommitted operations.
func (s *Sink) Abort(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.aborted = true
	return nil
}
