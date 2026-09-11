package rowmodel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"
)

// DomainKey names a source ordering domain within a stable source incarnation.
// Consumer, run and attempt identities must not be used as incarnations.
type DomainKey struct{ Incarnation, Domain string }

// Validate requires nonempty UTF-8 incarnation and domain identifiers.
func (d DomainKey) Validate() error {
	if d.Incarnation == "" || d.Domain == "" || !utf8.ValidString(d.Incarnation) || !utf8.ValidString(d.Domain) {
		return errors.New("progress: missing incarnation or domain")
	}
	return nil
}

// Position stores a codec-encoded source bookmark, independently of message content.
type Position struct {
	Codec   string
	Version int
	// Value is the codec-encoded source position, not message content.
	Value []byte
}

// Clone returns a position with independently owned encoded bytes.
func (p Position) Clone() Position { p.Value = bytes.Clone(p.Value); return p }

// Validate checks codec identity and version; the codec validates Value.
func (p Position) Validate() error {
	if p.Codec == "" || p.Version < 0 || !utf8.ValidString(p.Codec) {
		return errors.New("progress: missing codec or negative version")
	}
	return nil
}

// PositionOrder describes codec-defined progress ordering; zero fails closed.
// Encoded byte ordering does not establish source progress.
type PositionOrder uint8

// Position comparison outcomes fail closed when ordering cannot be established.
const (
	PositionIncomparable PositionOrder = iota
	PositionEqual
	PositionBefore
	PositionAfter
)

// ErrPositionIncomparable indicates that compatible progress ordering cannot be established.
var ErrPositionIncomparable = errors.New("progress: positions incomparable")

// PositionCodec validates, compares, and canonicalizes one source position format.
// Implementations must be deterministic, pure, and safe for concurrent use.
type PositionCodec interface {
	Validate(Position) error
	Compare(a, b Position) (PositionOrder, error)
	Canonicalize(Position) (Position, error)
}

// CodecResolver locates a position codec by identifier and version.
type CodecResolver interface {
	Lookup(codec string, version int) (PositionCodec, error)
}

// CanonicalPosition validates and canonicalizes a position without retaining caller buffers.
func CanonicalPosition(r CodecResolver, p Position) (Position, error) {
	if err := p.Validate(); err != nil {
		return Position{}, err
	}
	if r == nil {
		return Position{}, errors.New("progress: codec resolver required")
	}
	codec, err := r.Lookup(p.Codec, p.Version)
	if err != nil {
		return Position{}, err
	}
	if codec == nil {
		return Position{}, errors.New("progress: nil codec")
	}
	if err := codec.Validate(p.Clone()); err != nil {
		return Position{}, err
	}
	out, err := codec.Canonicalize(p.Clone())
	if err != nil {
		return Position{}, err
	}
	if out.Codec != p.Codec || out.Version != p.Version {
		return Position{}, errors.New("progress: canonicalization changed codec identity")
	}
	if err := codec.Validate(out.Clone()); err != nil {
		return Position{}, err
	}
	return out.Clone(), nil
}

// ComparePositions compares compatible domains and codecs, failing closed on incomparable progress.
func ComparePositions(r CodecResolver, aDomain DomainKey, a Position, bDomain DomainKey, b Position) (PositionOrder, error) {
	if aDomain.Validate() != nil || bDomain.Validate() != nil || aDomain != bDomain || a.Codec != b.Codec || a.Version != b.Version {
		return PositionIncomparable, ErrPositionIncomparable
	}
	a, err := CanonicalPosition(r, a)
	if err != nil {
		return PositionIncomparable, err
	}
	b, err = CanonicalPosition(r, b)
	if err != nil {
		return PositionIncomparable, err
	}
	codec, err := r.Lookup(a.Codec, a.Version)
	if err != nil {
		return PositionIncomparable, err
	}
	order, err := codec.Compare(a, b)
	if err != nil {
		return PositionIncomparable, err
	}
	if order == PositionIncomparable || order > PositionAfter {
		return PositionIncomparable, ErrPositionIncomparable
	}
	return order, nil
}

// DomainPosition pairs a source domain with its bookmark for wire serialization.
type DomainPosition struct {
	Domain   DomainKey
	Position Position
}

// DomainPositions uses sorted entries on the wire; duplicate domains are invalid.
// Clone when handing ownership across a session/coordinator boundary.
type DomainPositions map[DomainKey]Position

// Clone returns an independently owned map and position values.
func (p DomainPositions) Clone() DomainPositions {
	if p == nil {
		return nil
	}
	out := make(DomainPositions, len(p))
	for k, v := range p {
		out[k] = v.Clone()
	}
	return out
}

// Entries validates and copies positions into deterministic incarnation/domain order.
func (p DomainPositions) Entries() ([]DomainPosition, error) {
	out := make([]DomainPosition, 0, len(p))
	for k, v := range p {
		if err := k.Validate(); err != nil {
			return nil, err
		}
		if err := v.Validate(); err != nil {
			return nil, err
		}
		out = append(out, DomainPosition{k, v.Clone()})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Domain, out[j].Domain
		if a.Incarnation != b.Incarnation {
			return a.Incarnation < b.Incarnation
		}
		return a.Domain < b.Domain
	})
	return out, nil
}

// MarshalJSON encodes sorted entries instead of a map with struct keys.
func (p DomainPositions) MarshalJSON() ([]byte, error) {
	entries, err := p.Entries()
	if err != nil {
		return nil, err
	}
	return json.Marshal(entries)
}

// UnmarshalJSON validates entries and rejects duplicate domains before replacing the map.
func (p *DomainPositions) UnmarshalJSON(data []byte) error {
	var entries []DomainPosition
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	out := make(DomainPositions, len(entries))
	for _, e := range entries {
		if err := e.Domain.Validate(); err != nil {
			return err
		}
		if err := e.Position.Validate(); err != nil {
			return err
		}
		if _, ok := out[e.Domain]; ok {
			return fmt.Errorf("progress: duplicate domain %q", e.Domain.Domain)
		}
		out[e.Domain] = e.Position.Clone()
	}
	*p = out
	return nil
}
