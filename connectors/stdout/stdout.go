// Package stdout implements the ingestion.Sink interface by printing batches as NDJSON.
// It is the simplest concrete sink: it serializes each record to its own JSON line,
// computes the write-side CRC the engine verifies against the read CRC, and on
// Commit prints a one-line manifest summarizing the run. It holds no buffering or
// transactional state beyond per-run accounting, so Abort is a no-op marker — useful
// as the reference a real object-store sink is checked against.
//
// It is a development sink, so it serializes records itself rather than emitting the
// integrity canonical encoding (which is a hash preimage, not guaranteed valid JSON)
// — keeping the hot CRC path free of per-record JSON validation.
package stdout

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"

	ingestion "github.com/galaxy-io/filament"
)

// Sink writes batches as NDJSON to an io.Writer (os.Stdout by default).
type Sink struct {
	mu      sync.Mutex
	w       io.Writer
	run     ingestion.RunID
	acct    map[string]*resourceAcct
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
	s := &Sink{w: os.Stdout, acct: map[string]*resourceAcct{}}
	for _, o := range opts {
		o(s)
	}
	return s
}

var _ ingestion.Sink = (*Sink)(nil)

// Spec describes the sink's write capabilities.
func (s *Sink) Spec() ingestion.SinkSpec {
	return ingestion.SinkSpec{
		Name:        "stdout",
		DisplayName: "Standard Output (NDJSON)",
		Version:     "1",
		Capabilities: ingestion.SinkCapabilities{WritePolicies: ingestion.WriteCapabilities(
			ingestion.IngestionAppend,
			ingestion.IngestionSnapshotReplace,
		)},
	}
}

// Name identifies this sink implementation.
func (s *Sink) Name() string { return "stdout" }

// Open begins a run, resetting per-run accounting.
func (s *Sink) Open(_ context.Context, run ingestion.RunSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.run = run.Run
	s.acct = map[string]*resourceAcct{}
	s.aborted = false
	return nil
}

// Write prints each record as one canonical NDJSON line and returns a receipt
// whose WriteCRC is computed over the same records the batcher hashed for ReadCRC
// — so the engine's integrity check passes unless the stream write itself failed.
func (s *Sink) Write(_ context.Context, b ingestion.Batch) (ingestion.WriteReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var nbytes int64
	for i := range b.Records {
		line, err := encodeRecord(b.Records[i])
		if err != nil {
			return ingestion.WriteReceipt{}, fmt.Errorf("stdout: encode %s: %w", b.Resource, err)
		}
		if _, err := s.w.Write(line); err != nil {
			return ingestion.WriteReceipt{}, fmt.Errorf("stdout: write %s: %w", b.Resource, err)
		}
		if _, err := s.w.Write(newline); err != nil {
			return ingestion.WriteReceipt{}, fmt.Errorf("stdout: write %s: %w", b.Resource, err)
		}
		nbytes += int64(len(line)) + 1
	}

	a := s.acct[b.Resource]
	if a == nil {
		a = &resourceAcct{}
		s.acct[b.Resource] = a
	}
	a.records += int64(len(b.Records))
	a.bytes += nbytes
	a.batches++

	crc, _ := ingestion.CRC32C(b.Records)
	return ingestion.WriteReceipt{
		URI:      fmt.Sprintf("stdout://%s/%s", s.run, b.Resource),
		Bytes:    nbytes,
		Rows:     len(b.Records),
		WriteCRC: crc,
	}, nil
}

// Apply validates the batch against the run's write policy, then delegates to Write.
func (s *Sink) Apply(ctx context.Context, b ingestion.Batch, opts ingestion.ApplyOptions) (ingestion.WriteReceipt, error) {
	switch opts.Policy.Capability.Mode {
	case ingestion.WriteAppend, ingestion.WriteReplace:
		if err := opts.Policy.ValidateRecords(b.Resource, b.Records); err != nil {
			return ingestion.WriteReceipt{}, fmt.Errorf("stdout: %w", err)
		}
		return s.Write(ctx, b)
	default:
		return ingestion.WriteReceipt{}, fmt.Errorf("stdout: write policy %q is not implemented", opts.Policy.Capability.Mode)
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

var newline = []byte{'\n'}

// recordLine is the dev-readable JSON shape emitted per record.
type recordLine struct {
	ID   string          `json:"id"`
	Op   int             `json:"op"`
	Data json.RawMessage `json:"data"`
}

// encodeRecord serializes one record to a JSON line. A payload that is already valid
// JSON is embedded raw; anything else is emitted as a JSON string so the line is
// always valid JSON. This is the stdout sink's own concern — the integrity CRC uses
// the canonical encoding (ingestion.CRC32C), which does no such validation.
func encodeRecord(r ingestion.Record) ([]byte, error) {
	data := json.RawMessage(r.Data)
	if !json.Valid(r.Data) {
		s, err := json.Marshal(string(r.Data))
		if err != nil {
			return nil, err
		}
		data = s
	}
	return json.Marshal(recordLine{ID: r.ID, Op: int(r.Op), Data: data})
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
