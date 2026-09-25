package streamkit

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

// ErrBoundaryClosed reports use after a boundary completed or its tracker closed.
var ErrBoundaryClosed = errors.New("streamkit: boundary closed")

// BoundaryTracker requests one Read's soft boundary. It belongs to the producer
// owner: Add, Requested, Control and Close must be serialized with row writing.
// Wake only wakes that owner; no timer goroutine touches a builder. The connector
// still supplies authoritative source controls and complete candidate coverage.
type BoundaryTracker struct {
	limits            filament.Boundary
	started, last     time.Time
	records           int
	bytes             int64
	timer             *time.Timer
	requested, closed bool
	err               error
	transactions      map[rowmodel.DomainKey]string
}

// NewBoundaryTracker starts idle and age clocks for one Read. Zero disables a
// limit; negative limits are invalid. Create a new tracker for the next Read.
func NewBoundaryTracker(limits filament.Boundary) (*BoundaryTracker, error) {
	if limits.MaxRecords < 0 || limits.MaxBytes < 0 || limits.MaxWait < 0 || limits.MaxAge < 0 {
		return nil, errors.New("streamkit: negative boundary limit")
	}
	now := time.Now()
	b := &BoundaryTracker{limits: limits, started: now, last: now, transactions: make(map[rowmodel.DomainKey]string)}
	b.arm()
	return b, nil
}

// Wake returns the timer channel to select alongside native input. After a wake,
// inspect Requested and seek a safe source boundary, even if no new row arrives.
// It is nil when time limits are disabled. Count limits are checked after Add.
func (b *BoundaryTracker) Wake() <-chan time.Time {
	if b.timer == nil {
		return nil
	}
	return b.timer.C
}

// Add accounts for completed source records and their connector-measured bytes.
// Call after successful row projection. A request is sticky; subsequent records
// may exceed a soft target to finish a transaction. Zero records do not reset idle.
func (b *BoundaryTracker) Add(records int, nbytes int64) error {
	if err := b.ready(); err != nil {
		return err
	}
	if records < 0 || nbytes < 0 || records > math.MaxInt-b.records || nbytes > math.MaxInt64-b.bytes {
		return b.fail(errors.New("streamkit: invalid boundary accounting"))
	}
	b.Requested() // Latch an expired idle deadline before updating activity.
	b.records += records
	b.bytes += nbytes
	if records > 0 {
		b.last = time.Now()
	}
	if !b.Requested() {
		b.arm()
	}
	return nil
}

// Requested reports whether any soft limit has been reached. This is a request
// to finish at a safe source boundary, never permission to truncate a transaction.
func (b *BoundaryTracker) Requested() bool {
	if b.closed || b.err != nil {
		return false
	}
	now := time.Now()
	b.requested = b.requested ||
		(b.limits.MaxRecords > 0 && b.records >= b.limits.MaxRecords) ||
		(b.limits.MaxBytes > 0 && b.bytes >= b.limits.MaxBytes) ||
		(b.limits.MaxWait > 0 && now.Sub(b.last) >= b.limits.MaxWait) ||
		(b.limits.MaxAge > 0 && now.Sub(b.started) >= b.limits.MaxAge)
	return b.requested
}

// Request asks the owner to finish at the next safe source boundary, for example
// after receiving a native seal command or reaching end-of-input. It does not
// flush or cancel anything and must run on the same owner as Add and Control.
func (b *BoundaryTracker) Request() error {
	if err := b.ready(); err != nil {
		return err
	}
	b.requested = true
	if b.timer != nil {
		b.timer.Stop()
	}
	return nil
}

// Control forwards source evidence synchronously on the producer owner. The
// inlet must flush affected builders and wait for preceding Apply completion.
// True means a requested boundary completed at TxnEnd or ProgressBoundary with
// no tracked transaction open. It proves neither source-prefix completeness nor
// durability. Cancellation or any control error permanently fails this tracker.
func (b *BoundaryTracker) Control(ctx context.Context, sink filament.StreamRecordSink, c rowmodel.Control) (bool, error) {
	if err := b.ready(); err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, b.fail(err)
	}
	if err := b.validateControl(c); err != nil {
		return false, b.fail(err)
	}
	if err := sink.Control(ctx, c); err != nil {
		return false, b.fail(err)
	}
	if err := ctx.Err(); err != nil {
		return false, b.fail(err)
	}
	switch c.Kind {
	case rowmodel.TxnBegin:
		b.transactions[c.Domain] = c.TxnID
	case rowmodel.TxnEnd:
		delete(b.transactions, c.Domain)
	}
	safe := c.Kind == rowmodel.TxnEnd || c.Kind == rowmodel.ProgressBoundary
	if safe && len(b.transactions) == 0 && b.Requested() {
		b.Close()
		return true, nil
	}
	return false, nil
}

// Close stops the timer. It neither emits a control nor completes coverage.
func (b *BoundaryTracker) Close() {
	b.closed = true
	if b.timer != nil {
		b.timer.Stop()
	}
}

func (b *BoundaryTracker) ready() error {
	if b.err != nil {
		return b.err
	}
	if b.closed {
		return ErrBoundaryClosed
	}
	return nil
}

func (b *BoundaryTracker) fail(err error) error { b.err = err; b.Close(); return err }

func (b *BoundaryTracker) arm() {
	var deadline time.Time
	if b.limits.MaxWait > 0 {
		deadline = b.last.Add(b.limits.MaxWait)
	}
	if b.limits.MaxAge > 0 {
		age := b.started.Add(b.limits.MaxAge)
		if deadline.IsZero() || age.Before(deadline) {
			deadline = age
		}
	}
	if deadline.IsZero() {
		return
	}
	if b.timer == nil {
		b.timer = time.NewTimer(time.Until(deadline))
	} else {
		b.timer.Reset(time.Until(deadline))
	}
}

func (b *BoundaryTracker) validateControl(c rowmodel.Control) error {
	if err := c.Validate(); err != nil {
		return err
	}
	txn := b.transactions[c.Domain]
	switch c.Kind {
	case rowmodel.TxnBegin:
		if txn != "" {
			return errors.New("streamkit: nested transaction")
		}
	case rowmodel.TxnEnd:
		if txn != c.TxnID {
			return errors.New("streamkit: unmatched transaction end")
		}
	case rowmodel.ProgressBoundary, rowmodel.MemberActivated:
		if txn != "" {
			return errors.New("streamkit: progress inside transaction")
		}
	}
	return nil
}
