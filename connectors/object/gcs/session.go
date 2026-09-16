package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	object "github.com/galaxy-io/filament/connectors/object/internal"
)

const manifestChunkSize = 256 << 10

// uploadSession owns the bounded buffers and remote state for one sink run.
// It deliberately knows nothing about Filament batches or write policies.
type uploadSession struct {
	mu        sync.Mutex
	store     resumableStore
	bucket    string
	chunkSize int
	metadata  objectMetadata
	resources map[string]*objectWriter
	slots     chan struct{}
	encoded   sync.Pool
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	closeErr  error
}

// objectWriter owns one ordered GCS byte stream. Apply hands encoded buffers to
// requests; the run-owned goroutine releases them after the SDK has consumed
// them. This is the GCS equivalent of handing a full part to an S3 uploader.
type objectWriter struct {
	appendMu  sync.Mutex
	mu        sync.Mutex
	resource  string
	key       string
	writer    resumableWriter
	requests  chan appendRequest
	done      chan struct{}
	finish    sync.Once
	rows      int64
	bytes     int64
	crc32c    uint32
	remoteCRC uint32
	err       error
	closed    bool
	completed bool
}

type appendRequest struct{ data []byte }

func newUploadSession(ctx context.Context, store resumableStore, cfg sinkConfig) *uploadSession {
	// The engine cancels its extraction context before committing a resumable
	// pause. Once Apply hands a buffer off, the run session owns that upload;
	// Commit/Abort and their contexts still bound all session-owned work.
	sessionCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s := &uploadSession{
		store: store, bucket: cfg.bucket, chunkSize: cfg.chunkSize,
		metadata:  objectMetadata{contentType: cfg.fileFormat.ContentType(), contentEncoding: cfg.compression.ContentEncoding()},
		resources: make(map[string]*objectWriter), slots: make(chan struct{}, cfg.uploadWorkers),
		ctx: sessionCtx, cancel: cancel,
	}
	s.encoded.New = func() any {
		buffer := make([]byte, 0, initialEncodedBufferCap)
		return &buffer
	}
	return s
}

func (s *uploadSession) takeEncodedBuffer() []byte { return (*s.encoded.Get().(*[]byte))[:0] }

func (s *uploadSession) releaseEncodedBuffer(buffer []byte) {
	if cap(buffer) <= maxPooledEncodedBuffer {
		buffer = buffer[:0]
		s.encoded.Put(&buffer)
	}
}

func (s *uploadSession) resource(resource, key string) *objectWriter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if writer := s.resources[resource]; writer != nil {
		return writer
	}
	writer := &objectWriter{
		resource: resource,
		key:      key,
		writer:   s.store.NewWriter(s.ctx, s.bucket, key, s.metadata, s.chunkSize),
		requests: make(chan appendRequest),
		done:     make(chan struct{}),
	}
	s.resources[resource] = writer
	go s.upload(writer)
	return writer
}

func (s *uploadSession) upload(writer *objectWriter) {
	defer close(writer.done)
	for {
		select {
		case request, ok := <-writer.requests:
			if !ok {
				s.completeWriter(writer)
				return
			}
			err := s.withSlot(s.ctx, func() error { return writeAll(writer.writer, request.data) })
			s.releaseEncodedBuffer(request.data)
			if err != nil {
				writer.setError(err)
				writer.writer.Abort(err)
				s.Cancel()
				return
			}
		case <-s.ctx.Done():
			err := s.ctx.Err()
			writer.setError(err)
			writer.writer.Abort(err)
			return
		}
	}
}

func (s *uploadSession) completeWriter(writer *objectWriter) {
	err := s.withSlot(s.ctx, writer.writer.Close)
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.closed = true
	if err != nil {
		if writer.err == nil {
			writer.err = err
		}
		s.Cancel()
		return
	}
	writer.remoteCRC = writer.writer.CRC32C()
	writer.completed = true
}

func (w *objectWriter) setError(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err == nil {
		w.err = err
	}
}

func (s *uploadSession) acquire(ctx context.Context) error {
	select {
	case s.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

func (s *uploadSession) withSlot(ctx context.Context, operation func() error) error {
	if err := s.acquire(ctx); err != nil {
		return err
	}
	defer func() { <-s.slots }()
	return operation()
}

func (s *uploadSession) operationContext(ctx context.Context) (context.Context, func()) {
	opCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, cancel)
	return opCtx, func() {
		stop()
		cancel()
	}
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

// Append consumes data on every return. Once the unbuffered handoff succeeds,
// the session owns the upload independently of the Apply context.
func (s *uploadSession) Append(
	ctx context.Context,
	resource string,
	key string,
	data []byte,
	rows int,
	encodedCRC uint32,
) error {
	writer := s.resource(resource, key)
	writer.appendMu.Lock()
	defer writer.appendMu.Unlock()

	writer.mu.Lock()
	closed, uploadErr := writer.closed, writer.err
	writer.mu.Unlock()
	if closed {
		s.releaseEncodedBuffer(data)
		return fmt.Errorf("resource upload is closed")
	}
	if uploadErr != nil {
		s.releaseEncodedBuffer(data)
		return uploadErr
	}

	select {
	case writer.requests <- appendRequest{data: data}:
		writer.mu.Lock()
		writer.rows += int64(rows)
		writer.bytes += int64(len(data))
		writer.crc32c = object.CombineCRC32C(writer.crc32c, encodedCRC, int64(len(data)))
		writer.mu.Unlock()
		return nil
	case <-ctx.Done():
		s.releaseEncodedBuffer(data)
		return ctx.Err()
	case <-s.ctx.Done():
		s.releaseEncodedBuffer(data)
		return s.ctx.Err()
	}
}

func (s *uploadSession) snapshot() []*objectWriter {
	s.mu.Lock()
	resources := make([]*objectWriter, 0, len(s.resources))
	for _, writer := range s.resources {
		resources = append(resources, writer)
	}
	s.mu.Unlock()
	sort.Slice(resources, func(i, j int) bool { return resources[i].key < resources[j].key })
	return resources
}

func (w *objectWriter) closeInput() {
	w.appendMu.Lock()
	w.finish.Do(func() {
		w.mu.Lock()
		w.closed = true
		w.mu.Unlock()
		close(w.requests)
	})
	w.appendMu.Unlock()
}

// Complete closes every resumable writer and verifies the locally composed
// encoded CRC against the checksum returned by GCS.
func (s *uploadSession) Complete(ctx context.Context) ([]object.ResourceResult, error) {
	resources := s.snapshot()
	for _, writer := range resources {
		writer.closeInput()
	}
	for _, writer := range resources {
		select {
		case <-writer.done:
		case <-ctx.Done():
			s.Cancel()
			return nil, ctx.Err()
		}
	}

	results := make([]object.ResourceResult, len(resources))
	for i, writer := range resources {
		result, err := writer.result()
		if err != nil {
			return nil, fmt.Errorf("gcs sink: commit %s: %w", writer.resource, err)
		}
		results[i] = result
	}
	return results, nil
}

func (w *objectWriter) result() (object.ResourceResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return object.ResourceResult{}, w.err
	}
	if !w.completed {
		return object.ResourceResult{}, fmt.Errorf("resource upload did not complete")
	}
	if w.crc32c != w.remoteCRC {
		return object.ResourceResult{}, fmt.Errorf("CRC32C mismatch: encoded %08x, GCS %08x", w.crc32c, w.remoteCRC)
	}
	return object.ResourceResult{Resource: w.resource, Key: w.key, Rows: w.rows, Bytes: w.bytes, CRC32C: w.remoteCRC}, nil
}

// PutObject shares the session-wide request limit with resumable uploads.
func (s *uploadSession) PutObject(ctx context.Context, key string, metadata objectMetadata, body []byte) error {
	opCtx, done := s.operationContext(ctx)
	defer done()
	return s.withSlot(opCtx, func() error {
		writer := s.store.NewWriter(opCtx, s.bucket, key, metadata, manifestChunkSize)
		if err := writeAll(writer, body); err != nil {
			writer.Abort(err)
			return err
		}
		return writer.Close()
	})
}

// Cancel stops session-owned asynchronous work without closing the store.
func (s *uploadSession) Cancel() { s.cancel() }

func (s *uploadSession) closeStore() error {
	s.closeOnce.Do(func() { s.closeErr = s.store.Close() })
	return s.closeErr
}

// Abort cancels every resumable writer and waits within the supplied cleanup
// context. GCS does not publish an object whose writer is closed with an error.
func (s *uploadSession) Abort(ctx context.Context) error {
	s.Cancel()
	resources := s.snapshot()
	for _, writer := range resources {
		writer.closeInput()
	}
	var errs []error
	for _, writer := range resources {
		select {
		case <-writer.done:
		case <-ctx.Done():
			errs = append(errs, ctx.Err())
			return errors.Join(append(errs, s.closeStore())...)
		}
	}
	return errors.Join(s.closeStore())
}
