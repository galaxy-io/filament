package streamkit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
	"unicode/utf8"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// EventIdentity is independent of runs, generations, attempts and consumer names.
// Ordinal distinguishes changes sharing a transaction position without overflow
// at uint32. Sources must supply stable event positions, including on replay.
type EventIdentity struct {
	Domain   rowmodel.DomainKey
	Position rowmodel.Position
	Ordinal  uint64
}

func (e EventIdentity) CanonicalBytes(r rowmodel.CodecResolver) ([]byte, error) {
	if err := e.Domain.Validate(); err != nil {
		return nil, err
	}
	if !utf8.ValidString(e.Domain.Incarnation) || !utf8.ValidString(e.Domain.Domain) || !utf8.ValidString(e.Position.Codec) {
		return nil, ErrInvalidEnvelope
	}
	p, err := rowmodel.CanonicalPosition(r, e.Position)
	if err != nil {
		return nil, err
	}
	e.Position = p
	return json.Marshal(struct {
		Version  int
		Identity EventIdentity
	}{1, e})
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
	if wire.Version != 1 {
		return EventIdentity{}, ErrInvalidEnvelope
	}
	canonical, err := wire.Identity.CanonicalBytes(r)
	if err != nil {
		return EventIdentity{}, err
	}
	if !bytes.Equal(data, canonical) {
		return EventIdentity{}, ErrInvalidEnvelope
	}
	return wire.Identity, nil
}

func (e EventIdentity) ID(r rowmodel.CodecResolver) (string, error) {
	data, err := e.CanonicalBytes(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "v1:" + hex.EncodeToString(sum[:]), nil
}

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
// fields are appended first; EndRow projects every envelope before batching and
// before any outer audit writer appends legacy lineage. No last-row Meta is used
// to reconstruct earlier envelopes. A projector is owned by one producer loop.
type Projector struct {
	arrowbatch.RowWriter
	codecs rowmodel.CodecResolver
}

func NewProjector(w arrowbatch.RowWriter, r rowmodel.CodecResolver) *Projector {
	return &Projector{w, r}
}
func WithEnvelopeFields(s rowmodel.Schema) (rowmodel.Schema, error) {
	return rowmodel.WithEnvelopeFields(s)
}
func (w *Projector) EndRow(e Envelope, meta rowmodel.Meta) error {
	// Validate before appending any SDK column. On error the source must abandon
	// the pending row/writer; RowWriter does not provide rollback of source fields.
	if (e.KeyNull && len(e.Key) > 0) || (e.PayloadNull && len(e.Payload) > 0) {
		return ErrInvalidEnvelope
	}
	identity, err := e.Identity.CanonicalBytes(w.codecs)
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
	w.String("v1:" + hex.EncodeToString(sum[:]))
	w.Bytes(identity)
	if e.Timestamp == nil {
		w.Null()
	} else {
		w.String(timestamp)
	}
	w.Bytes(headers)
	if e.KeyNull {
		w.Null()
	} else {
		w.Bytes(e.Key)
	}
	if e.PayloadNull {
		w.Null()
	} else {
		w.Bytes(e.Payload)
	}
	meta.Domain = e.Identity.Domain
	meta.Position = e.Identity.Position.Clone()
	meta.Ordinal = e.Identity.Ordinal
	return w.RowWriter.EndRow(meta)
}
