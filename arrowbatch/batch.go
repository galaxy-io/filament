package arrowbatch

import (
	"strconv"
	"sync/atomic"

	"github.com/apache/arrow-go/v18/arrow"

	"github.com/galaxy-io/filament/rowmodel"
)

// Batch owns one Arrow record batch and its operation vector. A newly created
// Batch has one reference. Rows and Operations are borrowed views valid only
// while the caller owns a reference to Batch.
//
// Batch must be passed by pointer. On successful handoff, ownership transfers to
// the receiver; callers retain ownership when a handoff returns an error.
type Batch struct {
	refs atomic.Int64
	rows arrow.RecordBatch
	ops  Operations

	Resource string
	Part     int
	Seq      uint64
	Cursor   *rowmodel.CheckpointData
	Drained  bool
	Last     rowmodel.Meta
}

// NewBatch takes ownership of rows and ops. Ops must not be mutated afterward.
func NewBatch(rows arrow.RecordBatch, ops Operations) *Batch {
	b := &Batch{rows: rows, ops: ops}
	b.refs.Store(1)
	return b
}

// NewMarker constructs a rowless, owned completion marker.
func NewMarker() *Batch {
	b := &Batch{Drained: true}
	b.refs.Store(1)
	return b
}

// Rows returns a borrowed Arrow record batch, valid until the caller releases
// its reference to Batch.
func (b *Batch) Rows() arrow.RecordBatch { return b.rows }

// Operations returns a borrowed, immutable operation vector. Nil means inserts.
func (b *Batch) Operations() Operations { return b.ops }

// NumRows returns the number of rows, or zero for a marker/released batch.
func (b *Batch) NumRows() int {
	if b == nil || b.rows == nil {
		return 0
	}
	return int(b.rows.NumRows())
}

// Op returns row i's operation.
func (b *Batch) Op(i int) rowmodel.Operation {
	if b.ops.Len() == 0 {
		return rowmodel.OpInsert
	}
	return b.ops.At(i)
}

// SeqString returns Seq as a base-10 token for subjects and dedup keys.
func (b *Batch) SeqString() string { return strconv.FormatUint(b.Seq, 10) }

// Retain adds an ownership reference. Retaining an already released batch is a
// programming error and panics deterministically instead of reviving freed Arrow
// memory.
func (b *Batch) Retain() *Batch {
	for {
		refs := b.refs.Load()
		if refs <= 0 {
			panic("arrowbatch: retain after release")
		}
		if b.refs.CompareAndSwap(refs, refs+1) {
			return b
		}
	}
}

// Release drops an ownership reference and releases Arrow memory exactly once.
func (b *Batch) Release() {
	refs := b.refs.Add(-1)
	if refs < 0 {
		panic("arrowbatch: release underflow")
	}
	if refs == 0 {
		if b.rows != nil {
			b.rows.Release()
			b.rows = nil
		}
		b.ops = Operations{}
	}
}
