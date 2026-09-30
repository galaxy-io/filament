package bigquery

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/bigquery/storage/apiv1/storagepb"
	"github.com/galaxy-io/filament"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func receiveOffset(t *testing.T, requests <-chan int64) int64 {
	t.Helper()
	select {
	case offset := <-requests:
		return offset
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a pipelined request")
		return -1
	}
}
func assertNoRequest(t *testing.T, requests <-chan int64) {
	t.Helper()
	select {
	case offset := <-requests:
		t.Fatalf("request at offset %d exceeded the outstanding-request limit", offset)
	case <-time.After(50 * time.Millisecond):
	}
}
func TestSlidingWindowRefillsBeforeAllAcknowledgements(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "one")
	permits := make(chan struct{}, 1)
	fake.ackPermits = permits
	fake.requests = make(chan int64, 32)
	for i := 0; i < 10; i++ {
		applyBatch(t, sink, "one", 3)
	}
	// Four requests must reach the server while it acknowledges none.
	for i := 0; i < streamInflightRequests; i++ {
		if offset := receiveOffset(t, fake.requests); offset != int64(i*3) {
			t.Fatalf("offset=%d", offset)
		}
	}
	assertNoRequest(t, fake.requests)
	permits <- struct{}{}
	// One acknowledgement refills one slot, even though the other three are
	// still waiting. A window-at-a-time implementation also fails this test.
	if offset := receiveOffset(t, fake.requests); offset != 12 {
		t.Fatalf("next offset=%d, want 12", offset)
	}
	assertNoRequest(t, fake.requests)
	close(permits)
	if err := sink.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !sink.memory.TryAcquire(queuedBytes) {
		t.Fatal("payload memory leaked")
	}
	sink.memory.Release(queuedBytes)
	if !sink.uploads.TryAcquire(uploadConcurrency) {
		t.Fatal("upload permits leaked")
	}
	sink.uploads.Release(uploadConcurrency)
}
func TestPipelinedRetryReplaysOnlyUnacknowledgedSuffix(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "one")
	permits := make(chan struct{}, 1)
	fake.ackPermits = permits
	fake.requests = make(chan int64, 32)
	fake.disconnectAfter = 5
	for i := 0; i < 8; i++ {
		applyBatch(t, sink, "one", 3)
	}
	for i := 0; i < 4; i++ {
		receiveOffset(t, fake.requests)
	}
	permits <- struct{}{} // offset zero becomes durable in the stream
	if offset := receiveOffset(t, fake.requests); offset != 12 {
		t.Fatalf("offset=%d", offset)
	}
	// Server persists offset 12 then drops the connection. Offsets 3,6,9,12
	// are replayed and answered ALREADY_EXISTS; offset zero must stay retired.
	if offset := receiveOffset(t, fake.requests); offset != 3 {
		t.Fatalf("retry started at %d, want unacknowledged offset 3", offset)
	}
	close(permits)
	if err := sink.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	countZero := 0
	for _, offset := range fake.requestOffsets {
		if offset == 0 {
			countZero++
		}
	}
	if countZero != 1 {
		t.Fatalf("acknowledged prefix replayed %d times", countZero)
	}
	for _, rows := range fake.streams {
		if rows != 24 {
			t.Fatalf("rows=%d, expected 24 without duplicates", rows)
		}
	}
	for _, ordinals := range fake.ordinals {
		for i, ordinal := range ordinals {
			if ordinal != int64(i) {
				t.Fatalf("ordinal[%d]=%d", i, ordinal)
			}
		}
	}
	if !sink.memory.TryAcquire(queuedBytes) {
		t.Fatal("retry leaked payload memory")
	}
	sink.memory.Release(queuedBytes)
}
func TestGlobalInflightLimitAndAbortReleasePendingRequests(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "one", "two", "three")
	fake.ackPermits = make(chan struct{})
	fake.requests = make(chan int64, 32)
	for _, resource := range []string{"one", "two", "three"} {
		for i := 0; i < 6; i++ {
			applyBatch(t, sink, resource, 3)
		}
	}
	for i := 0; i < uploadConcurrency; i++ {
		receiveOffset(t, fake.requests)
	}
	assertNoRequest(t, fake.requests)
	done := make(chan struct{})
	go func() { _ = sink.Abort(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Abort stuck with pending acknowledgements")
	}
	if !sink.memory.TryAcquire(queuedBytes) {
		t.Fatal("Abort leaked memory")
	}
	sink.memory.Release(queuedBytes)
	if !sink.uploads.TryAcquire(uploadConcurrency) {
		t.Fatal("Abort leaked concurrency permits")
	}
	sink.uploads.Release(uploadConcurrency)
}
func TestAppendAcknowledgementValidation(t *testing.T) {
	duplicate := &storagepb.AppendRowsResponse{Response: &storagepb.AppendRowsResponse_Error{Error: status.New(codes.AlreadyExists, "duplicate").Proto()}}
	if err := validateAppendResponse(duplicate, 3, false); err == nil {
		t.Fatal("accepted unexpected duplicate on first attempt")
	}
	if err := validateAppendResponse(duplicate, 3, true); err != nil {
		t.Fatal(err)
	}
	wrong := &storagepb.AppendRowsResponse{Response: &storagepb.AppendRowsResponse_AppendResult_{AppendResult: &storagepb.AppendRowsResponse_AppendResult{Offset: wrapperspb.Int64(4)}}}
	if err := validateAppendResponse(wrong, 3, false); err == nil {
		t.Fatal("accepted out-of-order acknowledgement")
	}
	invalid := &storagepb.AppendRowsResponse{Response: &storagepb.AppendRowsResponse_Error{Error: status.New(codes.InvalidArgument, "bad schema").Proto()}}
	if err := validateAppendResponse(invalid, 3, true); err == nil || retryableAppend(err) {
		t.Fatal("permanent error accepted or retryable")
	}
}

func TestPermanentAppendFailureDoesNotPublishAndReleasesMemory(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "one")
	fake.rejectAppends = true
	applyBatch(t, sink, "one", 3)
	if err := sink.Commit(context.Background()); err == nil {
		t.Fatal("accepted permanently rejected upload")
	}
	if err := sink.Abort(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.statements) > 0 || fake.commits > 0 {
		t.Fatal("published rejected stream")
	}
	if !sink.memory.TryAcquire(queuedBytes) {
		t.Fatal("failure leaked payload memory")
	}
	sink.memory.Release(queuedBytes)
	if !sink.uploads.TryAcquire(uploadConcurrency) {
		t.Fatal("failure leaked upload permits")
	}
	sink.uploads.Release(uploadConcurrency)
}
