package s3

import (
	"context"
	"fmt"
	"sync"

	object "github.com/galaxy-io/filament/connectors/object/internal"
)

const (
	maxUploadParts         = 10_000
	maxPartSize            = maxPartSizeMiB << 20
	maxPooledEncodedBuffer = 4 << 20
	// partsPerTier is how many parts are written at one size before the part
	// size doubles. Ten tiers of 1,000 parts starting at the 5 MiB floor reach
	// roughly 5 TiB, the S3 object size limit, inside the 10,000-part ceiling.
	partsPerTier = 1_000
)

// multipartSession owns the bounded buffers and remote state for one sink run.
// It deliberately knows nothing about Filament batches or write policies.
type multipartSession struct {
	mu       sync.Mutex
	store    multipartStore
	bucket   string
	partSize int
	perTier  int32
	workers  int
	objects  map[string]*objectWriter // by object key; a resource may roll into several
	slots    chan struct{}
	buffers  *bufferPool
	encoded  sync.Pool
	metadata objectMetadata
	ctx      context.Context
	cancel   context.CancelFunc
}

// objectWriter owns the ordered byte stream and remote multipart state for one
// object. Different objects proceed independently; appends to one object are
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

func newMultipartSession(
	ctx context.Context,
	store multipartStore,
	bucket string,
	partSize int64,
	workers int,
	metadata objectMetadata,
) *multipartSession {
	// The engine cancels its extraction context before committing a resumable
	// pause. Apply/Commit contexts and Cancel still bound every operation.
	sessionCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	session := &multipartSession{
		store: store, bucket: bucket, partSize: int(partSize), perTier: partsPerTier, workers: workers,
		objects:  make(map[string]*objectWriter),
		slots:    make(chan struct{}, workers),
		buffers:  newBufferPool(),
		metadata: metadata,
		ctx:      sessionCtx,
		cancel:   cancel,
	}
	session.encoded.New = func() any {
		buffer := make([]byte, 0, bufferChunkSize)
		return &buffer
	}
	return session
}

func newObjectWriter(resource, key string) *objectWriter {
	return &objectWriter{resource: resource, key: key}
}

// Append adds bytes to one object and hands every full part to an uploader.
func (s *multipartSession) Append(ctx context.Context, resource, key string, data []byte, rows int, encodedCRC uint32) error {
	upload := s.object(resource, key)
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
			upload.buffer = newPartBuffer(s.buffers, s.partLimit(upload.partNum+1))
		}
		count := upload.buffer.Append(data)
		data = data[count:]
		if !upload.buffer.Full() {
			upload.mu.Unlock()
			continue
		}

		// The next buffer is created lazily once this part has its number, so
		// its size follows the schedule for the part it will become.
		body := upload.buffer
		upload.buffer = nil
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
	upload.crc32c = object.CombineCRC32C(upload.crc32c, encodedCRC, int64(len(payload)))
	upload.rows += int64(rows)
	upload.bytes += int64(len(payload))
	upload.mu.Unlock()
	return nil
}

func (s *multipartSession) object(resource, key string) *objectWriter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if upload := s.objects[key]; upload != nil {
		return upload
	}
	upload := newObjectWriter(resource, key)
	s.objects[key] = upload
	return upload
}

// Seal closes one object to further appends and starts uploading its tail now
// instead of at Commit, so a resource that rolled to a new object stops holding
// the old one's buffer. The multipart upload stays open, and the object stays
// invisible, until Complete. A sealed object that never filled a part becomes
// a one-part multipart upload, which S3 accepts at any size.
func (s *multipartSession) Seal(ctx context.Context, key string) error {
	s.mu.Lock()
	upload := s.objects[key]
	s.mu.Unlock()
	if upload == nil {
		return nil
	}
	upload.appendMu.Lock()
	defer upload.appendMu.Unlock()

	upload.mu.Lock()
	if upload.err != nil {
		err := upload.err
		upload.mu.Unlock()
		return err
	}
	upload.closed = true
	body := upload.buffer
	upload.buffer = nil
	if body == nil || body.Len() == 0 {
		upload.mu.Unlock()
		body.Release()
		return nil
	}
	if upload.uploadID == "" {
		upload.mu.Unlock()
		uploadID, err := s.createMultipart(ctx, upload.key)
		upload.mu.Lock()
		if err != nil {
			body.Release()
			upload.err = fmt.Errorf("create multipart upload: %w", err)
			err = upload.err
			upload.mu.Unlock()
			return err
		}
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
	return s.startPartUpload(ctx, upload, uploadPart{number: number, body: body})
}

// partLimit returns the size of multipart part number n. The configured part
// size holds for the first perTier parts and doubles every perTier parts after
// that, capped at the S3 per-part maximum. The object's total size is unknown
// while it streams, so growing the parts is what lets a large object finish
// inside the 10,000-part ceiling without reserving huge buffers for small ones.
// S3 accepts parts of different sizes as long as every part but the last is at
// least 5 MiB, which the configured floor already guarantees.
func (s *multipartSession) partLimit(n int32) int {
	return partLimit(s.partSize, n, s.perTier)
}

func partLimit(base int, n, perTier int32) int {
	if n < 1 {
		n = 1
	}
	size := int64(base)
	for tier := (n - 1) / perTier; tier > 0 && size < maxPartSize; tier-- {
		size <<= 1
	}
	return int(min(size, maxPartSize))
}

func (u *objectWriter) nextPartNumber() (int32, error) {
	if u.partNum >= maxUploadParts {
		return 0, fmt.Errorf("multipart upload exceeds %d parts after %d bytes", maxUploadParts, u.bytes)
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
		uploadID, err = s.store.CreateMultipart(ctx, s.bucket, key, s.metadata)
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
		size, crc := int64(part.body.Len()), part.body.CRC32C()
		token, err := s.store.UploadPart(uploadCtx, s.bucket, upload.key, uploadID, part.number, part.body, size, crc)
		part.body.Release()
		upload.mu.Lock()
		defer upload.mu.Unlock()
		if err != nil {
			if upload.err == nil {
				upload.err = fmt.Errorf("upload part %d: %w", part.number, err)
			}
			return
		}
		upload.parts = append(upload.parts, completedPart{number: part.number, token: token, crc32c: crc})
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
