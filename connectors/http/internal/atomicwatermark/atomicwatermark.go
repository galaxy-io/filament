// Package atomicwatermark provides a lock-free max-watermark cell for
// incremental extraction.
//
// Watermark.Observe uses a CAS loop so the stored value is always the max
// per the supplied Comparator regardless of caller concurrency.
//
// Comparators are pluggable. Each returns an error on values it cannot order
// so a malformed timestamp surfaces immediately instead of corrupting state.
package atomicwatermark

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/galaxy-io/filament/connectors/http/errs"
)

// Comparator orders watermark candidates. Less reports a < b; the returned
// error indicates an unparseable input (Watermark.Observe propagates).
type Comparator interface {
	Less(a, b string) (bool, error)
	Name() string
}

// Watermark holds the max-observed value. The zero Watermark has no comparator
// and panics on use; construct with New.
type Watermark struct {
	cmp Comparator
	val atomic.Pointer[string]
}

// New constructs a Watermark with the given comparator and initial value.
// initial is treated as already observed (so the first Observe with a smaller
// value won't advance). Empty initial means "no value yet". Returns an error
// if initial is non-empty but unparseable by cmp — callers should not seed
// state that the comparator can't read back.
func New(cmp Comparator, initial string) (*Watermark, error) {
	if cmp == nil {
		panic("atomicwatermark.New: comparator is required")
	}
	w := &Watermark{cmp: cmp}
	if initial != "" {
		if _, err := cmp.Less(initial, initial); err != nil {
			return nil, fmt.Errorf("atomicwatermark: initial %q invalid for %s comparator: %w",
				initial, cmp.Name(), err)
		}
		s := initial
		w.val.Store(&s)
	}
	return w, nil
}

// Observe records v if it strictly exceeds the current value per cmp.
// Returns advanced=true when the stored value changed. Empty v is a no-op.
//
// v is validated against the comparator before any storage attempt — even on
// the very first observe — so an unparseable value never poisons the cell.
func (w *Watermark) Observe(v string) (advanced bool, err error) {
	if v == "" {
		return false, nil
	}
	// Self-compare validates parseability without depending on stored state,
	// so an unparseable value can't reach storage even on the first Observe.
	if _, err := w.cmp.Less(v, v); err != nil {
		return false, err
	}
	for {
		cur := w.val.Load()
		if cur != nil {
			less, err := w.cmp.Less(*cur, v)
			if err != nil {
				return false, err
			}
			if !less {
				return false, nil
			}
		}
		nv := v
		if w.val.CompareAndSwap(cur, &nv) {
			return true, nil
		}
		// CAS lost — another goroutine advanced; loop and re-evaluate.
	}
}

// Current returns the stored watermark value, or "" if never observed.
func (w *Watermark) Current() string {
	p := w.val.Load()
	if p == nil {
		return ""
	}
	return *p
}

// ComparatorName returns the human-readable name of the configured comparator.
func (w *Watermark) ComparatorName() string { return w.cmp.Name() }

// Lex compares strings byte-wise. Correct for RFC3339 timestamps and
// zero-padded ids; incorrect for raw integer strings ("9" > "100" lex).
type Lex struct{}

func (Lex) Less(a, b string) (bool, error) { return a < b, nil }
func (Lex) Name() string                   { return "lex" }

// Numeric parses both sides as float64 and compares numerically. Returns
// ErrPathType-wrapped errors on parse failure (no silent lex fallback).
type Numeric struct{}

func (Numeric) Less(a, b string) (bool, error) {
	af, err := strconv.ParseFloat(a, 64)
	if err != nil {
		return false, fmt.Errorf("%w: numeric watermark: %q is not a number", errs.ErrPathType, a)
	}
	bf, err := strconv.ParseFloat(b, 64)
	if err != nil {
		return false, fmt.Errorf("%w: numeric watermark: %q is not a number", errs.ErrPathType, b)
	}
	return af < bf, nil
}

func (Numeric) Name() string { return "numeric" }

// Time parses both sides as RFC3339 and compares chronologically. Errors on
// parse failure (no silent lex fallback).
type Time struct{}

func (Time) Less(a, b string) (bool, error) {
	at, err := time.Parse(time.RFC3339, a)
	if err != nil {
		return false, fmt.Errorf("%w: time watermark: %q not RFC3339", errs.ErrPathType, a)
	}
	bt, err := time.Parse(time.RFC3339, b)
	if err != nil {
		return false, fmt.Errorf("%w: time watermark: %q not RFC3339", errs.ErrPathType, b)
	}
	return at.Before(bt), nil
}

func (Time) Name() string { return "time" }

// ForName returns the named comparator. Returns an error for unknown names so
// manifest validation can fail loudly at load time. Accepted names: "lex"
// (default when empty), "numeric", "time".
func ForName(name string) (Comparator, error) {
	switch name {
	case "", "lex":
		return Lex{}, nil
	case "numeric":
		return Numeric{}, nil
	case "time":
		return Time{}, nil
	}
	return nil, fmt.Errorf("atomicwatermark: unknown comparator %q (want lex|numeric|time)", name)
}
