// Package stdout implements the filament.Sink interface by printing batches as NDJSON.
// It is the simplest concrete sink: it renders each row as its own JSON line,
// computes the write-side CRC the engine verifies against the read CRC, and on
// Commit prints a one-line manifest summarizing the run. It holds no buffering or
// transactional state beyond per-run accounting, so Abort is a no-op marker — useful
// as the reference a real object-store sink is checked against.
package stdout

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"

	"github.com/apache/arrow-go/v18/arrow"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/internal/ndjson"
)

// Sink writes batches as NDJSON to an io.Writer (os.Stdout by default).
type Sink struct {
	mu      sync.Mutex
	w       io.Writer
	run     filament.RunID
	acct    map[string]*resourceAcct
	enc     map[*arrow.Schema]*ndjson.Encoder
	buf     []byte
	aborted bool
}

// resourceAcct is the per-resource running tally used for the commit manifest.
type resourceAcct struct {
	records int64
	bytes   int64
	batches int
}

// Option configures a Sink.
type Option func(*Sink)

// WithWriter directs output to w instead of os.Stdout (used in tests).
func WithWriter(w io.Writer) Option { return func(s *Sink) { s.w = w } }

// New returns a stdout sink. By default it writes to os.Stdout.
func New(opts ...Option) *Sink {
	s := &Sink{w: os.Stdout, acct: map[string]*resourceAcct{}, enc: map[*arrow.Schema]*ndjson.Encoder{}}
	for _, o := range opts {
		o(s)
	}
	return s
}

var _ filament.Sink = (*Sink)(nil)

// Spec describes the sink's write capabilities.
func (s *Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         "stdout",
		DisplayName:  "Standard Output (NDJSON)",
		Description:  "Standard output stream for writing pipeline logs and output directly to the terminal.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-stdout-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-stdout-light.svg",
		Version:      "1",
		Capabilities: filament.SinkCapabilities{
			EncodedIntegrity: true,
			WritePolicies: filament.WriteCapabilities(
				filament.IngestionFullAppend,
				filament.IngestionFullReplace,
			),
		},
	}
}

// Name identifies this sink implementation.
func (s *Sink) Name() string { return "stdout" }

// Open begins a run, resetting per-run accounting.
func (s *Sink) Open(_ context.Context, run filament.RunSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.run = run.Run
	s.acct = map[string]*resourceAcct{}
	s.enc = map[*arrow.Schema]*ndjson.Encoder{}
	s.aborted = false
	return nil
}

// Write prints each row as one NDJSON line and returns a receipt whose WriteCRC
// is computed over the same batch the writer hashed for ReadCRC — so the engine's
// integrity check passes unless the stream write itself failed.
func (s *Sink) Write(_ context.Context, b *arrowbatch.Batch) (filament.WriteReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows := b.Rows()
	enc := s.enc[rows.Schema()]
	if enc == nil {
		enc = ndjson.NewEncoder(rows.Schema())
		s.enc[rows.Schema()] = enc
	}
	buf, expectedCRC := enc.AppendBatch(s.buf[:0], rows)
	// Capture the in-memory checksum at the same final boundary. The pipeline's
	// earlier checksum detects mutation during handoff; the encoded comparison
	// below detects mutation introduced by serialization.
	arrowCRC := b.IntegrityCRC()
	encodedCRC, err := ndjson.VerifyChecksum(buf, expectedCRC)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("stdout: serialize %s: %w", b.Resource, err)
	}
	s.buf = buf
	if _, err := s.w.Write(buf); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("stdout: write %s: %w", b.Resource, err)
	}
	nbytes := int64(len(buf))
	a := s.acct[b.Resource]
	if a == nil {
		a = &resourceAcct{}
		s.acct[b.Resource] = a
	}
	a.records += int64(b.NumRows())
	a.bytes += nbytes
	a.batches++

	return filament.WriteReceipt{
		URI:        fmt.Sprintf("stdout://%s/%s", s.run, b.Resource),
		Bytes:      nbytes,
		Rows:       b.NumRows(),
		WriteCRC:   arrowCRC,
		EncodedCRC: &encodedCRC,
	}, nil
}

// Apply validates the batch against the run's write policy, then delegates to Write.
func (s *Sink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	switch opts.Policy.Capability.Mode {
	case filament.WriteAppend, filament.WriteReplace:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("stdout: %w", err)
		}
		return s.Write(ctx, b)
	default:
		return filament.WriteReceipt{}, fmt.Errorf("stdout: write policy %q is not implemented", opts.Policy.Capability.Mode)
	}
}

// Commit prints a one-line JSON manifest summarizing the run, prefixed with '#'
// so it is distinguishable from the NDJSON data lines on the same stream.
func (s *Sink) Commit(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	names := make([]string, 0, len(s.acct))
	for name := range s.acct {
		names = append(names, name)
	}
	sort.Strings(names)

	m := manifest{Sink: "stdout", Run: string(s.run)}
	for _, name := range names {
		a := s.acct[name]
		m.Resources = append(m.Resources, manifestResource{
			Name: name, URI: fmt.Sprintf("stdout://%s/%s", s.run, name),
			Records: a.records, Bytes: a.bytes,
		})
	}
	enc, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("stdout: marshal manifest: %w", err)
	}
	if _, err := fmt.Fprintf(s.w, "# %s\n", enc); err != nil {
		return fmt.Errorf("stdout: write manifest: %w", err)
	}
	return nil
}

// Abort marks the run aborted; stdout has nothing to roll back.
func (s *Sink) Abort(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aborted = true
	return nil
}

// manifest is the commit summary printed to the stream.
type manifest struct {
	Sink      string             `json:"sink"`
	Run       string             `json:"run"`
	Resources []manifestResource `json:"resources"`
}

type manifestResource struct {
	Name    string `json:"name"`
	URI     string `json:"uri"`
	Records int64  `json:"records"`
	Bytes   int64  `json:"bytes"`
}
