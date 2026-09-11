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

func (d DomainKey) Validate() error {
	if d.Incarnation == "" || d.Domain == "" || !utf8.ValidString(d.Incarnation) || !utf8.ValidString(d.Domain) {
		return errors.New("progress: missing incarnation or domain")
	}
	return nil
}

type Position struct {
	Codec   string
	Version int
	// Value is the codec-encoded source position, not message content.
	Value []byte
}

func (p Position) Clone() Position { p.Value = bytes.Clone(p.Value); return p }
func (p Position) Validate() error {
	if p.Codec == "" || p.Version < 0 || !utf8.ValidString(p.Codec) {
		return errors.New("progress: missing codec or negative version")
	}
	return nil
}

// Zero fails closed. Ordering is connector-specific, never byte ordering.
type PositionOrder uint8

const (
	PositionIncomparable PositionOrder = iota
	PositionEqual
	PositionBefore
	PositionAfter
)

var ErrPositionIncomparable = errors.New("progress: positions incomparable")

type PositionCodec interface {
	Validate(Position) error
	Compare(a, b Position) (PositionOrder, error)
	Canonicalize(Position) (Position, error)
}
type CodecResolver interface {
	Lookup(codec string, version int) (PositionCodec, error)
}

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

type DomainPosition struct {
	Domain   DomainKey
	Position Position
}

// DomainPositions uses sorted entries on the wire; duplicate domains are invalid.
// Clone when handing ownership across a session/coordinator boundary.
type DomainPositions map[DomainKey]Position

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
func (p DomainPositions) MarshalJSON() ([]byte, error) {
	entries, err := p.Entries()
	if err != nil {
		return nil, err
	}
	return json.Marshal(entries)
}
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
