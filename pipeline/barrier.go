package pipeline

import (
	"context"
	"errors"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// BarrierReceipt identifies a control whose preceding writes passed Apply and
// integrity verification. Sequence is local to this pipeline. This is neither a
// sink commit receipt nor proof that the source supplied complete coverage.
type BarrierReceipt struct {
	Sequence uint64
	Boundary rowmodel.Control
}

type queuedBatch struct {
	batch    *arrowbatch.Batch
	complete chan BarrierReceipt
}

// Barrier flushes builders on their owning goroutine, queues an owned control,
// and waits for preceding writes. Call only on a NewStream pipeline, after Start,
// serialized with all inlet and row-writer operations. The pipeline stays open
// for subsequent epochs. Cancellation fails the pipeline; it is not a boundary.
func (p *Pipeline) Barrier(ctx context.Context, control rowmodel.Control) (BarrierReceipt, error) {
	if p.stream == nil {
		return BarrierReceipt{}, errors.New("pipeline: barriers require NewStream")
	}
	in := p.stream
	if err := in.ready(); err != nil {
		return BarrierReceipt{}, err
	}
	if err := ctx.Err(); err != nil {
		return BarrierReceipt{}, in.fail(err)
	}
	if err := in.validateControl(control); err != nil {
		return BarrierReceipt{}, in.fail(err)
	}
	b, err := arrowbatch.NewControl(control)
	if err != nil {
		return BarrierReceipt{}, in.fail(err)
	}
	// A flush may block on backpressure before the control can be enqueued.
	stop := context.AfterFunc(ctx, func() { p.setErr(ctx.Err()) })
	defer stop()
	if err := in.flush(); err != nil {
		b.Release()
		return BarrierReceipt{}, in.fail(err)
	}
	in.sequence++
	b.Seq = in.sequence
	complete := make(chan BarrierReceipt, 1)
	select {
	case p.batchCh <- queuedBatch{batch: b, complete: complete}:
	case <-p.done:
		b.Release()
		return BarrierReceipt{}, in.failure()
	}
	select {
	case receipt := <-complete:
		if err := ctx.Err(); err != nil {
			return BarrierReceipt{}, in.fail(err)
		}
		if err := in.ready(); err != nil {
			return BarrierReceipt{}, err
		}
		in.acceptControl(control)
		return receipt, nil
	case <-p.done:
		return BarrierReceipt{}, in.failure()
	}
}
