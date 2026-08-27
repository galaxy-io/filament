package s3

import (
	"context"
	"fmt"
	"sync"
)

const (
	maxUploadParts         = 10_000
	maxPooledEncodedBuffer = 4 << 20
	ndjsonContentType      = "application/x-ndjson"
)

// multipartSession owns the bounded buffers and remote state for one sink run.
// It deliberately knows nothing about Filament batches or write policies.
type multipartSession struct {
	mu        sync.Mutex
	store     multipartStore
	bucket    string
	partSize  int
	workers   int
	resources map[string]*objectWriter
	slots     chan struct{}
	buffers   *bufferPool
	encoded   sync.Pool
	ctx       context.Context
	cancel    context.CancelFunc
}

// objectWriter owns the ordered byte stream and remote multipart state for one
// resource. Different objects proceed independently; appends to one object are
// serialized so part order and the manifest checksum stay deterministic.
type objectWriter struct {
	appendMu  sync.Mutex
	mu        sync.Mutex
	resource  string
	key       string
	uploadID  string
	buffer    *partBuffer
	parts     []completedPart
	partNum   int32
	inflight  sync.WaitGroup
	err       error
	rows      int64
	bytes     int64
	crc32c    uint32
	closed    bool
	completed bool
}

type uploadPart struct {
	number int32
	body   *partBuffer
}

type resourceResult struct {
	resource string
	key      string
	rows     int64
	bytes    int64
	crc32c   uint32
}

func newMultipartSession(
	ctx context.Context,
	store multipartStore,
	bucket string,
	partSize int64,
	workers int,
	declared map[string]string,
) *multipartSession {
	// The engine cancels its extraction context before committing a resumable
	// pause. Apply/Commit contexts and Cancel still bound every operation.
	sessionCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	session := &multipartSession{
		store: store, bucket: bucket, partSize: int(partSize), workers: workers,
		resources: make(map[string]*objectWriter, len(declared)),
		slots:     make(chan struct{}, workers),
		buffers:   newBufferPool(),
		ctx:       sessionCtx,
		cancel:    cancel,
	}
	session.encoded.New = func() any {
		buffer := make([]byte, 0, bufferChunkSize)
		return &buffer
	}
	for resource, key := range declared {
		session.resources[resource] = newObjectWriter(resource, key)
	}
	return session
}

func newObjectWriter(resource, key string) *objectWriter {
	return &objectWriter{resource: resource, key: key}
}

// Append adds bytes to one resource and hands every full part to an uploader.
func (s *multipartSession) Append(ctx context.Context, resource, key string, data []byte, rows int, encodedCRC uint32) error {
	upload := s.resource(resource, key)
	upload.appendMu.Lock()
	defer upload.appendMu.Unlock()

	upload.mu.Lock()
	if upload.closed {
		upload.mu.Unlock()
		return fmt.Errorf("resource upload is closed")
	}
	if upload.err != nil {
		err := upload.err
		upload.mu.Unlock()
		return err
	}
	upload.mu.Unlock()

	payload := data
	for len(data) > 0 {
		upload.mu.Lock()
		if upload.err != nil {
			err := upload.err
			upload.mu.Unlock()
			return err
		}
		if upload.buffer == nil {
			upload.buffer = newPartBuffer(s.buffers, s.partSize)
		}
		count := upload.buffer.Append(data)
		data = data[count:]
		if !upload.buffer.Full() {
			upload.mu.Unlock()
			continue
		}

		body := upload.buffer
		upload.buffer = newPartBuffer(s.buffers, s.partSize)
		if upload.uploadID == "" {
			upload.mu.Unlock()
			uploadID, err := s.createMultipart(ctx, upload.key)
			if err != nil {
				body.Release()
				upload.mu.Lock()
				upload.err = fmt.Errorf("create multipart upload: %w", err)
				err = upload.err
				upload.mu.Unlock()
				return err
			}
			upload.mu.Lock()
			upload.uploadID = uploadID
		}
		number, err := upload.nextPartNumber()
		if err != nil {
			body.Release()
			upload.err = err
			upload.mu.Unlock()
			return err
		}
		upload.mu.Unlock()

		if err := s.startPartUpload(ctx, upload, uploadPart{number: number, body: body}); err != nil {
			return err
		}
	}

	upload.mu.Lock()
	upload.crc32c = combineCRC32C(upload.crc32c, encodedCRC, int64(len(payload)))
	upload.rows += int64(rows)
	upload.bytes += int64(len(payload))
	upload.mu.Unlock()
	return nil
}

func (s *multipartSession) resource(resource, key string) *objectWriter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if upload := s.resources[resource]; upload != nil {
		return upload
	}
	upload := newObjectWriter(resource, key)
	s.resources[resource] = upload
	return upload
}

func (u *objectWriter) nextPartNumber() (int32, error) {
	if u.partNum >= maxUploadParts {
		return 0, fmt.Errorf("multipart upload exceeds %d parts; increase part_size_mib", maxUploadParts)
	}
	u.partNum++
	return u.partNum, nil
}

func (s *multipartSession) takeEncodedBuffer() []byte {
	return (*s.encoded.Get().(*[]byte))[:0]
}

func (s *multipartSession) releaseEncodedBuffer(buffer []byte) {
	if cap(buffer) <= maxPooledEncodedBuffer {
		buffer = buffer[:0]
		s.encoded.Put(&buffer)
	}
}

func (s *multipartSession) operationContext(ctx context.Context) (context.Context, func()) {
	opCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, cancel)
	return opCtx, func() {
		stop()
		cancel()
	}
}

func (s *multipartSession) acquireSlot(ctx context.Context) error {
	select {
	case s.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *multipartSession) withSlot(ctx context.Context, operation func(context.Context) error) error {
	if err := s.acquireSlot(ctx); err != nil {
		return err
	}
	defer func() { <-s.slots }()
	return operation(ctx)
}

func (s *multipartSession) createMultipart(ctx context.Context, key string) (string, error) {
	opCtx, done := s.operationContext(ctx)
	defer done()
	var uploadID string
	err := s.withSlot(opCtx, func(ctx context.Context) error {
		var err error
		uploadID, err = s.store.CreateMultipart(ctx, s.bucket, key, ndjsonContentType)
		return err
	})
	return uploadID, err
}

func (s *multipartSession) startPartUpload(ctx context.Context, upload *objectWriter, part uploadPart) error {
	waitCtx, stopWaiting := s.operationContext(ctx)
	if err := s.acquireSlot(waitCtx); err != nil {
		stopWaiting()
		part.body.Release()
		upload.setError(err)
		return err
	}
	stopWaiting()

	// Once Apply has handed a full part off, its request belongs to the run
	// session. The pipeline cancels its writer context after extraction drains,
	// before Sink.Commit waits for these requests; inheriting the Apply context
	// here would cancel valid in-flight parts at that lifecycle boundary.
	uploadCtx, stopUpload := context.WithCancel(s.ctx)

	// A previous asynchronous part may have failed while this call waited for
	// capacity. Do not start another request after the resource is poisoned.
	upload.mu.Lock()
	if upload.err != nil {
		err := upload.err
		upload.mu.Unlock()
		<-s.slots
		stopUpload()
		part.body.Release()
		return err
	}
	uploadID := upload.uploadID
	upload.mu.Unlock()

	upload.inflight.Go(func() {
		defer stopUpload()
		defer func() { <-s.slots }()
		size := int64(part.body.Len())
		etag, err := s.store.UploadPart(uploadCtx, s.bucket, upload.key, uploadID, part.number, part.body, size)
		part.body.Release()
		upload.mu.Lock()
		defer upload.mu.Unlock()
		if err != nil {
			if upload.err == nil {
				upload.err = fmt.Errorf("upload part %d: %w", part.number, err)
			}
			return
		}
		upload.parts = append(upload.parts, completedPart{number: part.number, etag: etag})
	})
	return nil
}

func (u *objectWriter) setError(err error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.err == nil {
		u.err = err
	}
}
