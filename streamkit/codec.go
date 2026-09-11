// Package streamkit provides optional public helpers for native stream connectors.
// It supplies encoding mechanics, never runtime admission or delivery guarantees.
package streamkit

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/galaxy-io/filament/rowmodel"
)

var ErrUnknownCodec = errors.New("streamkit: unknown codec")

type codecKey struct {
	name    string
	version int
}

// Registry is safe for concurrent lookup/registration. Codecs must be pure,
// deterministic and concurrency-safe. Registrations cannot replace live codecs.
type Registry struct {
	mu     sync.RWMutex
	codecs map[codecKey]rowmodel.PositionCodec
}

func (r *Registry) Register(name string, version int, c rowmodel.PositionCodec) error {
	if name == "" || version < 0 || c == nil {
		return errors.New("streamkit: invalid codec registration")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := codecKey{name, version}
	if _, ok := r.codecs[key]; ok {
		return errors.New("streamkit: duplicate codec")
	}
	if r.codecs == nil {
		r.codecs = make(map[codecKey]rowmodel.PositionCodec)
	}
	r.codecs[key] = c
	return nil
}
func (r *Registry) Lookup(name string, version int) (rowmodel.PositionCodec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.codecs[codecKey{name, version}]
	if !ok {
		return nil, fmt.Errorf("%w: %s v%d", ErrUnknownCodec, name, version)
	}
	return c, nil
}

// OpaqueCodec permits equality only; differing opaque cursors are incomparable.
// Nil and empty payloads remain distinct. It does not interpret native cursors.
type OpaqueCodec struct{}

func (OpaqueCodec) Validate(p rowmodel.Position) error { return p.Validate() }
func (c OpaqueCodec) Canonicalize(p rowmodel.Position) (rowmodel.Position, error) {
	return p.Clone(), c.Validate(p)
}
func (c OpaqueCodec) Compare(a, b rowmodel.Position) (rowmodel.PositionOrder, error) {
	if a.Codec != b.Codec || a.Version != b.Version {
		return rowmodel.PositionIncomparable, nil
	}
	if err := c.Validate(a); err != nil {
		return rowmodel.PositionIncomparable, err
	}
	if err := c.Validate(b); err != nil {
		return rowmodel.PositionIncomparable, err
	}
	if (a.Payload == nil) == (b.Payload == nil) && bytes.Equal(a.Payload, b.Payload) {
		return rowmodel.PositionEqual, nil
	}
	return rowmodel.PositionIncomparable, nil
}

// Uint64Codec encodes a connector-declared unsigned counter as decimal ASCII.
// Leading zeros normalize away. It must not be used for LSNs, GTID sets or opaque
// cursors merely because a sample happens to contain digits.
type Uint64Codec struct{}

func (Uint64Codec) Validate(p rowmodel.Position) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if len(p.Payload) == 0 {
		return errors.New("streamkit: empty counter")
	}
	for _, b := range p.Payload {
		if b < '0' || b > '9' {
			return errors.New("streamkit: non-decimal counter")
		}
	}
	_, err := strconv.ParseUint(string(p.Payload), 10, 64)
	return err
}
func (c Uint64Codec) Canonicalize(p rowmodel.Position) (rowmodel.Position, error) {
	if err := c.Validate(p); err != nil {
		return rowmodel.Position{}, err
	}
	n, _ := strconv.ParseUint(string(p.Payload), 10, 64)
	p.Payload = []byte(strconv.FormatUint(n, 10))
	return p, nil
}
func (c Uint64Codec) Compare(a, b rowmodel.Position) (rowmodel.PositionOrder, error) {
	if a.Codec != b.Codec || a.Version != b.Version {
		return rowmodel.PositionIncomparable, nil
	}
	if err := c.Validate(a); err != nil {
		return rowmodel.PositionIncomparable, err
	}
	if err := c.Validate(b); err != nil {
		return rowmodel.PositionIncomparable, err
	}
	x, _ := strconv.ParseUint(string(a.Payload), 10, 64)
	y, _ := strconv.ParseUint(string(b.Payload), 10, 64)
	if x < y {
		return rowmodel.PositionBefore, nil
	}
	if x > y {
		return rowmodel.PositionAfter, nil
	}
	return rowmodel.PositionEqual, nil
}
