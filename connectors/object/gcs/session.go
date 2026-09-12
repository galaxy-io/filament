package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	"cloud.google.com/go/storage"
)

type objectMetadata struct {
	contentType     string
	contentEncoding string
}

type resourceResult struct {
	resource string
	key      string
	rows     int64
	bytes    int64
	crc32c   uint32
}

type objectWriter struct {
	mu       sync.Mutex
	resource string
	key      string
	writer   *storage.Writer
	rows     int64
	bytes    int64
	crc32c   uint32
	err      error
	closed   bool
}

type uploadSession struct {
	mu        sync.Mutex
	client    *storage.Client
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

func newUploadSession(ctx context.Context, client *storage.Client, cfg sinkConfig) *uploadSession {
	sessionCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s := &uploadSession{
		client: client, bucket: cfg.bucket, chunkSize: cfg.chunkSize,
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
	w := s.client.Bucket(s.bucket).Object(key).NewWriter(s.ctx)
	w.ChunkSize = s.chunkSize
	w.ContentType = s.metadata.contentType
	w.ContentEncoding = s.metadata.contentEncoding
	writer := &objectWriter{resource: resource, key: key, writer: w}
	s.resources[resource] = writer
	return writer
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

func (s *uploadSession) Append(ctx context.Context, resource, key string, data []byte, rows int) error {
	object := s.resource(resource, key)
	object.mu.Lock()
	defer object.mu.Unlock()
	if object.closed {
		return fmt.Errorf("resource upload is closed")
	}
	if object.err != nil {
		return object.err
	}
	size := len(data)
	err := s.withSlot(ctx, func() error { return writeAll(object.writer, data) })
	if err != nil {
		object.err = err
		return err
	}
	object.rows += int64(rows)
	object.bytes += int64(size)
	return nil
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

func (s *uploadSession) Complete(ctx context.Context) ([]resourceResult, error) {
	resources := s.snapshot()
	if len(resources) == 0 {
		return nil, nil
	}
	type job struct {
		index  int
		writer *objectWriter
	}
	jobs := make(chan job)
	errs := make([]error, len(resources))
	completeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	for range min(cap(s.slots), len(resources)) {
		wg.Go(func() {
			for job := range jobs {
				if err := s.completeResource(completeCtx, job.writer); err != nil {
					errs[job.index] = fmt.Errorf("gcs sink: commit %s: %w", job.writer.resource, err)
					cancel()
					s.Cancel()
				}
			}
		})
	}

sendLoop:
	for i, writer := range resources {
		select {
		case jobs <- job{index: i, writer: writer}:
		case <-completeCtx.Done():
			break sendLoop
		}
	}
	close(jobs)
	wg.Wait()
	for _, err := range errs {
		if err != nil && !errors.Is(err, context.Canceled) {
			return nil, err
		}
	}
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	results := make([]resourceResult, len(resources))
	for i, writer := range resources {
		results[i] = writer.result()
	}
	return results, completeCtx.Err()
}

func (s *uploadSession) completeResource(ctx context.Context, object *objectWriter) error {
	object.mu.Lock()
	defer object.mu.Unlock()
	if object.err != nil {
		return object.err
	}
	if object.closed {
		return nil
	}
	object.closed = true
	if err := s.withSlot(ctx, object.writer.Close); err != nil {
		object.err = err
		return err
	}
	object.crc32c = object.writer.Attrs().CRC32C
	return nil
}

func (w *objectWriter) result() resourceResult {
	w.mu.Lock()
	defer w.mu.Unlock()
	return resourceResult{resource: w.resource, key: w.key, rows: w.rows, bytes: w.bytes, crc32c: w.crc32c}
}

func (s *uploadSession) PutObject(ctx context.Context, key string, metadata objectMetadata, body []byte) error {
	return s.withSlot(ctx, func() error {
		w := s.client.Bucket(s.bucket).Object(key).NewWriter(s.ctx)
		w.ChunkSize = 256 << 10
		w.ContentType = metadata.contentType
		w.ContentEncoding = metadata.contentEncoding
		if err := writeAll(w, body); err != nil {
			s.Cancel()
			_ = w.Close()
			return err
		}
		return w.Close()
	})
}

func (s *uploadSession) Cancel() { s.cancel() }

func (s *uploadSession) closeClient() error {
	s.closeOnce.Do(func() { s.closeErr = s.client.Close() })
	return s.closeErr
}

func (s *uploadSession) Abort() error {
	s.Cancel()
	for _, object := range s.snapshot() {
		object.mu.Lock()
		if !object.closed {
			object.closed = true
			_ = object.writer.Close()
		}
		object.mu.Unlock()
	}
	return s.closeClient()
}
