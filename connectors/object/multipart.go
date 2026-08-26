package s3

import (
	"context"
	"fmt"
	"hash"
	"hash/crc32"
	"sync"
)

const (
	maxUploadParts    = 10_000
	ndjsonContentType = "application/x-ndjson"
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

// multipartSession owns the bounded buffers and remote state for one sink run.
// It deliberately knows nothing about Filament batches or write policies.
type multipartSession struct {
	mu        sync.Mutex
	store     multipartStore
	bucket    string
	partSize  int
	workers   int
	resources map[string]*resourceUpload
	slots     chan struct{}
	pool      sync.Pool
	ctx       context.Context
	cancel    context.CancelFunc
}

type resourceUpload struct {
	appendMu  sync.Mutex
	mu        sync.Mutex
	resource  string
	key       string
	uploadID  string
	buffer    []byte
	parts     []completedPart
	partNum   int32
	inflight  sync.WaitGroup
	err       error
	rows      int64
	bytes     int64
	crc       hash.Hash32
	closed    bool
	completed bool
}

type uploadPart struct {
	number int32
	body   []byte
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
		resources: make(map[string]*resourceUpload, len(declared)),
		slots:     make(chan struct{}, workers),
		ctx:       sessionCtx,
		cancel:    cancel,
	}
	session.pool.New = func() any { return make([]byte, 0, session.partSize) }
	for resource, key := range declared {
		session.resources[resource] = newResourceUpload(resource, key)
	}
	return session
}

func newResourceUpload(resource, key string) *resourceUpload {
	return &resourceUpload{resource: resource, key: key, crc: crc32.New(crcTable)}
}

// Append adds bytes to one resource and hands every full part to an uploader.
func (s *multipartSession) Append(ctx context.Context, resource, key string, data []byte, rows int) error {
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
			upload.buffer = s.takeBuffer()
		}
		count := min(s.partSize-len(upload.buffer), len(data))
		upload.buffer = append(upload.buffer, data[:count]...)
		data = data[count:]
		if len(upload.buffer) != s.partSize {
			upload.mu.Unlock()
			continue
		}

		body := upload.buffer
		upload.buffer = s.takeBuffer()
		if upload.uploadID == "" {
			upload.mu.Unlock()
			uploadID, err := s.createMultipart(ctx, upload.key)
			if err != nil {
				s.releaseBuffer(body)
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
			s.releaseBuffer(body)
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
	_, _ = upload.crc.Write(payload)
	upload.rows += int64(rows)
	upload.bytes += int64(len(payload))
	upload.mu.Unlock()
	return nil
}

func (s *multipartSession) resource(resource, key string) *resourceUpload {
	s.mu.Lock()
	defer s.mu.Unlock()
	if upload := s.resources[resource]; upload != nil {
		return upload
	}
	upload := newResourceUpload(resource, key)
	s.resources[resource] = upload
	return upload
}

func (u *resourceUpload) nextPartNumber() (int32, error) {
	if u.partNum >= maxUploadParts {
		return 0, fmt.Errorf("multipart upload exceeds %d parts; increase part_size_mib", maxUploadParts)
	}
	u.partNum++
	return u.partNum, nil
}

func (s *multipartSession) takeBuffer() []byte { return s.pool.Get().([]byte)[:0] }

func (s *multipartSession) releaseBuffer(buffer []byte) {
	if buffer != nil {
		s.pool.Put(buffer[:0])
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

func (s *multipartSession) startPartUpload(ctx context.Context, upload *resourceUpload, part uploadPart) error {
	opCtx, done := s.operationContext(ctx)
	if err := s.acquireSlot(opCtx); err != nil {
		done()
		s.releaseBuffer(part.body)
		upload.setError(err)
		return err
	}

	// A previous asynchronous part may have failed while this call waited for
	// capacity. Do not start another request after the resource is poisoned.
	upload.mu.Lock()
	if upload.err != nil {
		err := upload.err
		upload.mu.Unlock()
		<-s.slots
		done()
		s.releaseBuffer(part.body)
		return err
	}
	uploadID := upload.uploadID
	upload.mu.Unlock()

	upload.inflight.Go(func() {
		defer done()
		defer func() { <-s.slots }()
		etag, err := s.store.UploadPart(opCtx, s.bucket, upload.key, uploadID, part.number, part.body)
		s.releaseBuffer(part.body)
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

func (u *resourceUpload) setError(err error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.err == nil {
		u.err = err
	}
}
