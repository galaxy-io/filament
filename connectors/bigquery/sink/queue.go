package bigquery

import (
	"context"
	"sync"
)

// uploadQueue is bounded by the sink's shared byte semaphore. An additional
// per-table batch-count limit would block the pipeline writer on one resource
// while other upload workers and most of the byte budget remain idle.
type uploadQueue struct {
	mu      sync.Mutex
	ready   *sync.Cond
	batches []uploadBatch
	head    int
	closed  bool
}

func newUploadQueue() *uploadQueue {
	q := &uploadQueue{}
	q.ready = sync.NewCond(&q.mu)
	return q
}
func (q *uploadQueue) push(batch uploadBatch) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.batches = append(q.batches, batch)
	q.ready.Signal()
}
func (q *uploadQueue) pop() (uploadBatch, bool) {
	batch, ok, _ := q.popContext(context.Background())
	return batch, ok
}
func (q *uploadQueue) popContext(ctx context.Context) (uploadBatch, bool, error) {
	stop := context.AfterFunc(ctx, func() { q.mu.Lock(); q.ready.Broadcast(); q.mu.Unlock() })
	defer stop()
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.head == len(q.batches) && !q.closed && ctx.Err() == nil {
		q.ready.Wait()
	}
	if err := ctx.Err(); err != nil {
		return uploadBatch{}, false, err
	}
	if q.head == len(q.batches) {
		return uploadBatch{}, false, nil
	}
	batch := q.batches[q.head]
	q.batches[q.head] = uploadBatch{}
	q.head++
	if q.head == len(q.batches) {
		q.batches = q.batches[:0]
		q.head = 0
	} else if q.head >= 256 && q.head >= len(q.batches)/2 {
		q.batches = append([]uploadBatch(nil), q.batches[q.head:]...)
		q.head = 0
	}
	return batch, true, nil
}
func (q *uploadQueue) close() {
	q.mu.Lock()
	q.closed = true
	q.ready.Broadcast()
	q.mu.Unlock()
}
