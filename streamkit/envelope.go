package streamkit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// IdentityEncodingVersion identifies the canonical event identity encoding.
const IdentityEncodingVersion = 1

// EventIdentity shares the neutral value type; encoding belongs to this SDK.
type EventIdentity = rowmodel.EventIdentity

// CanonicalIdentity encodes an identity with its codec-canonical position.
func CanonicalIdentity(e EventIdentity, r rowmodel.CodecResolver) ([]byte, error) {
	if err := e.Domain.Validate(); err != nil {
		return nil, err
	}
	p, err := rowmodel.CanonicalPosition(r, e.Position)
	if err != nil {
		return nil, err
	}
	e.Position = p
	return json.Marshal(struct {
		Version  int
		Identity EventIdentity
	}{IdentityEncodingVersion, e})
}

// DecodeIdentity accepts only the canonical versioned SDK representation. It
// rejects unknown versions, duplicate/unknown fields and ambiguous spellings.
func DecodeIdentity(data []byte, r rowmodel.CodecResolver) (EventIdentity, error) {
	var wire struct {
		Version  int
		Identity EventIdentity
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return EventIdentity{}, err
	}
	if wire.Version != IdentityEncodingVersion {
		return EventIdentity{}, ErrInvalidEnvelope
	}
	identity := wire.Identity
	canonical, err := CanonicalIdentity(identity, r)
	if err != nil {
		return EventIdentity{}, err
	}
	if !bytes.Equal(data, canonical) {
		return EventIdentity{}, ErrInvalidEnvelope
	}
	return identity, nil
}

// EventID returns the SHA-256 hex digest of the canonical source event identity.
func EventID(e EventIdentity, r rowmodel.CodecResolver) (string, error) {
	data, err := CanonicalIdentity(e, r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Envelope combines source identity with message content and lossless delivery metadata.
type Envelope struct {
	Identity    EventIdentity
	Timestamp   *time.Time
	Headers     []Header
	Key         []byte
	KeyNull     bool
	Payload     []byte
	PayloadNull bool
}

// Projector wraps a schema-ordered writer opened with WithEnvelopeFields. Source
// fields are appended first; EndEvent projects every envelope before batching and
// before any outer audit writer appends legacy lineage. No last-row Meta is used
// to reconstruct earlier envelopes. A projector is owned by one producer loop.
type Projector struct {
	writer arrowbatch.RowWriter
	codecs rowmodel.CodecResolver
}

// NewProjector binds an envelope writer to its position codecs.
// The writer must have been opened with WithEnvelopeFields.
func NewProjector(w arrowbatch.RowWriter, r rowmodel.CodecResolver) *Projector {
	return &Projector{w, r}
}

// WithEnvelopeFields appends the SDK schema suffix after rejecting reserved-name collisions.
func WithEnvelopeFields(s rowmodel.Schema) (rowmodel.Schema, error) {
	return rowmodel.WithEnvelopeFields(s)
}

// EndEvent projects envelope columns and derives stream metadata before completing the row.
func (w *Projector) EndEvent(e Envelope, meta rowmodel.Meta) error {
	// Validate before appending any SDK column. On error the source must abandon
	// the pending row/writer; RowWriter does not provide rollback of source fields.
	if (e.KeyNull && len(e.Key) > 0) || (e.PayloadNull && len(e.Payload) > 0) {
		return ErrInvalidEnvelope
	}
	identity, err := CanonicalIdentity(e.Identity, w.codecs)
	if err != nil {
		return err
	}
	headers, err := EncodeHeaders(e.Headers)
	if err != nil {
		return err
	}
	var timestamp string
	if e.Timestamp != nil {
		if e.Timestamp.UTC().Year() < 0 || e.Timestamp.UTC().Year() > 9999 {
			return ErrInvalidEnvelope
		}
		timestamp = e.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	sum := sha256.Sum256(identity)
	w.writer.String(hex.EncodeToString(sum[:]))
	w.writer.Bytes(identity)
	if e.Timestamp == nil {
		w.writer.Null()
	} else {
		w.writer.String(timestamp)
	}
	w.writer.Bytes(headers)
	if e.KeyNull {
		w.writer.Null()
	} else {
		w.writer.Bytes(e.Key)
	}
	if e.PayloadNull {
		w.writer.Null()
	} else {
		w.writer.Bytes(e.Payload)
	}
	meta.Stream = &rowmodel.StreamMeta{Identity: e.Identity.Clone()}
	return w.writer.EndRow(meta)
}
