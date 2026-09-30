package bigquery

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/bigquery/storage/apiv1/storagepb"
	"github.com/googleapis/gax-go/v2"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const streamInflightRequests = 4
const maxAppendAttempts = 6

// pendingAppend owns its encoded buffers until acknowledged. Offsets are
// assigned once and preserved, including across a connection replacement.
type pendingAppend struct {
	batch    uploadBatch
	offset   int64
	attempts int
}
type appendAttempt struct {
	pending  *pendingAppend
	started  time.Time
	sent     chan struct{}
	sendTime time.Duration // read only after sent closes
	slotHeld atomic.Bool
}
type streamUploader struct {
	sink       *Sink
	table      *tableState
	mu         sync.Mutex
	pending    []*pendingAppend
	nextOffset int64
}

func (s *Sink) uploadTable(state *tableState) {
	defer s.workers.Done()
	uploader := &streamUploader{sink: s, table: state}
	if err := uploader.run(); err != nil {
		s.fail(fmt.Errorf("bigquery sink: upload %s: %w", state.definition.name, err))
	}
	// Failed requests remain owned through retries. At terminal failure release
	// them, then drain queued payloads until the pipeline seals the sink.
	for _, pending := range uploader.pending {
		s.memory.Release(pending.batch.size())
	}
	for {
		batch, ok := state.queue.pop()
		if !ok {
			break
		}
		s.memory.Release(batch.size())
	}
}
func (u *streamUploader) run() error {
	first, ok, err := u.table.queue.popContext(u.sink.ctx)
	if err != nil || !ok {
		return err
	}
	u.pending = []*pendingAppend{{batch: first, offset: 0}}
	u.nextOffset = first.rows
	if u.table.stream == "" {
		if err := u.sink.setups.Acquire(u.sink.ctx, 1); err != nil {
			return err
		}
		err = u.sink.prepareUpload(u.sink.ctx, u.table)
		u.sink.setups.Release(1)
		if err != nil {
			return err
		}
	}
	retries := 0
	for {
		before := u.table.rows
		err = u.session()
		if err == nil {
			return nil
		}
		if u.sink.ctx.Err() != nil {
			return u.sink.ctx.Err()
		}
		if !retryableAppend(err) {
			return err
		}
		if u.table.rows > before {
			retries = 0
		}
		retries++
		slog.WarnContext(u.sink.ctx, "BigQuery append connection retry", "component", "bigquery", "event.name", "bigquery.write.retry", "run_id", u.sink.run, "resource", u.table.definition.name, "attempt", retries, "offset", u.table.rows, "pending_requests", len(u.pending), "error", err)
		if retries >= maxAppendAttempts {
			return fmt.Errorf("append retries exhausted: %w", err)
		}
		timer := time.NewTimer(time.Duration(100*(1<<(retries-1))) * time.Millisecond)
		select {
		case <-timer.C:
		case <-u.sink.ctx.Done():
			timer.Stop()
			return u.sink.ctx.Err()
		}
	}
}

// One goroutine sends and one receives on each bidi connection. A sliding
// window caps outstanding requests without waiting for an entire window to
// finish. On failure both stop before the unacknowledged suffix is replayed.
func (u *streamUploader) session() error {
	ctx, cancel := context.WithCancel(u.sink.ctx)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "x-goog-request-params", "write_stream="+url.QueryEscape(u.table.stream))
	connection, err := u.sink.storage.AppendRows(ctx, gax.WithGRPCOptions(grpc.UseCompressor(gzip.Name)))
	if err != nil {
		return err
	}
	defer connection.CloseSend()
	window := semaphore.NewWeighted(streamInflightRequests)
	issued := make(chan *appendAttempt, streamInflightRequests)
	replay := append([]*pendingAppend(nil), u.pending...)
	reserved := make(map[*appendAttempt]struct{}) // guarded by u.mu
	group, sessionCtx := errgroup.WithContext(ctx)
	// errgroup cancellation must cancel the gRPC stream as well as queue waits.
	stop := context.AfterFunc(sessionCtx, cancel)
	defer stop()
	group.Go(func() error {
		defer close(issued)
		first := true
		for {
			if err := window.Acquire(sessionCtx, 1); err != nil {
				return err
			}
			var pending *pendingAppend
			if len(replay) > 0 {
				pending = replay[0]
				replay[0] = nil // do not retain acknowledged payloads in the replay backing array
				replay = replay[1:]
			} else {
				batch, ok, err := u.table.queue.popContext(sessionCtx)
				if err != nil {
					return err
				}
				if !ok {
					return nil
				}
				pending = &pendingAppend{batch: batch, offset: u.nextOffset}
				u.nextOffset += batch.rows
				u.mu.Lock()
				u.pending = append(u.pending, pending)
				u.mu.Unlock()
			}
			if pending.attempts >= maxAppendAttempts {
				return fmt.Errorf("append at offset %d exhausted %d attempts", pending.offset, maxAppendAttempts)
			}
			waiting := time.Now()
			if err := u.sink.uploads.Acquire(sessionCtx, 1); err != nil {
				return err
			}
			u.sink.logTiming("upload_slot", u.table.definition.name, waiting)
			attempt := &appendAttempt{pending: pending, started: time.Now(), sent: make(chan struct{})}
			attempt.slotHeld.Store(true)
			u.mu.Lock()
			reserved[attempt] = struct{}{}
			u.mu.Unlock()
			pending.attempts++
			select {
			case issued <- attempt:
			case <-sessionCtx.Done():
				return sessionCtx.Err()
			}
			data := &storagepb.AppendRowsRequest_ArrowData{Rows: &storagepb.ArrowRecordBatch{SerializedRecordBatch: pending.batch.data, RowCount: pending.batch.rows}}
			if first {
				data.WriterSchema = &storagepb.ArrowSchema{SerializedSchema: pending.batch.schema}
				first = false
			}
			request := &storagepb.AppendRowsRequest{WriteStream: u.table.stream, Offset: wrapperspb.Int64(pending.offset), Rows: &storagepb.AppendRowsRequest_ArrowRows{ArrowRows: data}}
			started := time.Now()
			err := connection.Send(request)
			attempt.sendTime = time.Since(started)
			close(attempt.sent)
			if err != nil {
				return err
			}
		}
	})
	group.Go(func() error {
		for attempt := range issued {
			response, err := connection.Recv()
			if err != nil {
				return err
			}
			pending := attempt.pending
			if err := validateAppendResponse(response, pending.offset, pending.attempts > 1); err != nil {
				return err
			}
			// Send may still own the buffers even if the server has already replied.
			select {
			case <-attempt.sent:
			case <-sessionCtx.Done():
				return sessionCtx.Err()
			}
			u.mu.Lock()
			if len(u.pending) == 0 || u.pending[0] != pending {
				u.mu.Unlock()
				return fmt.Errorf("append acknowledgement is out of order")
			}
			delete(reserved, attempt)
			u.pending[0] = nil
			u.pending = u.pending[1:]
			u.table.rows += pending.batch.rows
			u.mu.Unlock()
			u.sink.memory.Release(pending.batch.size())
			if attempt.slotHeld.Swap(false) {
				u.sink.uploads.Release(1)
			}
			window.Release(1)
			u.sink.logTiming("append", u.table.definition.name, attempt.started, "records", pending.batch.rows, "bytes", pending.batch.size(), "offset", pending.offset, "compression", gzip.Name, "send_ms", attempt.sendTime.Milliseconds(), "response_wait_ms", (time.Since(attempt.started) - attempt.sendTime).Milliseconds())
		}
		return nil
	})
	err = group.Wait()
	// Requests whose acknowledgement was lost keep their payloads and offsets,
	// but release concurrency permits before the next connection acquires them.
	for attempt := range reserved {
		if attempt.slotHeld.Swap(false) {
			u.sink.uploads.Release(1)
		}
	}
	return err
}
func validateAppendResponse(response *storagepb.AppendRowsResponse, offset int64, retried bool) error {
	if failure := response.GetError(); failure != nil {
		err := status.ErrorProto(failure)
		if retried && status.Code(err) == codes.AlreadyExists {
			return nil
		}
		return err
	}
	if len(response.GetRowErrors()) > 0 {
		return fmt.Errorf("append rejected rows: %v", response.GetRowErrors())
	}
	result := response.GetAppendResult()
	if result == nil || result.Offset == nil || result.Offset.Value != offset {
		return fmt.Errorf("append returned an unexpected offset: expected %d", offset)
	}
	return nil
}
func retryableAppend(err error) bool {
	if err == io.EOF {
		return true
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.Internal, codes.Aborted, codes.DeadlineExceeded, codes.ResourceExhausted:
		return true
	}
	return false
}
