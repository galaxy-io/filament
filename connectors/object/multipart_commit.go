package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
)

// Complete finishes every resource and returns immutable manifest input.
func (s *multipartSession) Complete(ctx context.Context) ([]resourceResult, error) {
	resources := s.snapshot()
	if err := s.completeResources(ctx, resources); err != nil {
		return nil, err
	}
	results := make([]resourceResult, len(resources))
	for i, upload := range resources {
		results[i] = upload.result()
	}
	return results, nil
}

func (s *multipartSession) completeResources(ctx context.Context, resources []*objectWriter) error {
	if len(resources) == 0 {
		return nil
	}
	type job struct {
		index  int
		upload *objectWriter
	}
	jobs := make(chan job)
	errs := make([]error, len(resources))
	completeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for range min(s.workers, len(resources)) {
		wg.Go(func() {
			for job := range jobs {
				if err := s.completeResource(completeCtx, job.upload); err != nil {
					errs[job.index] = fmt.Errorf("s3 sink: commit %s: %w", job.upload.resource, err)
					cancel()
					s.Cancel()
				}
			}
		})
	}

sendLoop:
	for i, upload := range resources {
		select {
		case jobs <- job{index: i, upload: upload}:
		case <-completeCtx.Done():
			break sendLoop
		}
	}
	close(jobs)
	wg.Wait()
	for _, err := range errs {
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return completeCtx.Err()
}

func (s *multipartSession) completeResource(ctx context.Context, upload *objectWriter) error {
	upload.inflight.Wait()
	upload.mu.Lock()
	defer upload.mu.Unlock()
	upload.closed = true
	if upload.err != nil {
		return upload.err
	}
	if upload.uploadID == "" {
		var body io.ReadSeeker = bytes.NewReader(nil)
		var size int64
		if upload.buffer != nil {
			body = upload.buffer
			size = int64(upload.buffer.Len())
		}
		if err := s.PutObject(ctx, upload.key, ndjsonContentType, body, size); err != nil {
			return err
		}
		upload.completed = true
		upload.buffer.Release()
		upload.buffer = nil
		return nil
	}
	if upload.buffer != nil && upload.buffer.Len() > 0 {
		number, err := upload.nextPartNumber()
		if err != nil {
			return err
		}
		etag, err := s.uploadPart(ctx, upload.key, upload.uploadID, number, upload.buffer)
		if err != nil {
			return fmt.Errorf("upload tail part %d: %w", number, err)
		}
		upload.parts = append(upload.parts, completedPart{number: number, etag: etag})
	}
	sort.Slice(upload.parts, func(i, j int) bool { return upload.parts[i].number < upload.parts[j].number })
	if err := s.completeMultipart(ctx, upload.key, upload.uploadID, upload.parts); err != nil {
		return err
	}
	upload.completed = true
	upload.buffer.Release()
	upload.buffer = nil
	return nil
}

func (u *objectWriter) result() resourceResult {
	u.mu.Lock()
	defer u.mu.Unlock()
	return resourceResult{resource: u.resource, key: u.key, rows: u.rows, bytes: u.bytes, crc32c: u.crc32c}
}

func (s *multipartSession) uploadPart(ctx context.Context, key, uploadID string, number int32, body *partBuffer) (string, error) {
	opCtx, done := s.operationContext(ctx)
	defer done()
	var etag string
	size := int64(body.Len())
	err := s.withSlot(opCtx, func(ctx context.Context) error {
		var err error
		etag, err = s.store.UploadPart(ctx, s.bucket, key, uploadID, number, body, size)
		return err
	})
	return etag, err
}

func (s *multipartSession) completeMultipart(ctx context.Context, key, uploadID string, parts []completedPart) error {
	opCtx, done := s.operationContext(ctx)
	defer done()
	return s.withSlot(opCtx, func(ctx context.Context) error {
		return s.store.CompleteMultipart(ctx, s.bucket, key, uploadID, parts)
	})
}

// PutObject shares the session-wide request limit with multipart operations.
func (s *multipartSession) PutObject(ctx context.Context, key, contentType string, body io.ReadSeeker, size int64) error {
	opCtx, done := s.operationContext(ctx)
	defer done()
	return s.withSlot(opCtx, func(ctx context.Context) error {
		return s.store.PutObject(ctx, s.bucket, key, contentType, body, size)
	})
}

func (s *multipartSession) snapshot() []*objectWriter {
	s.mu.Lock()
	resources := make([]*objectWriter, 0, len(s.resources))
	for _, upload := range s.resources {
		resources = append(resources, upload)
	}
	s.mu.Unlock()
	sort.Slice(resources, func(i, j int) bool { return resources[i].key < resources[j].key })
	return resources
}

// Cancel stops session-owned asynchronous work without touching remote state.
func (s *multipartSession) Cancel() { s.cancel() }

// Abort cancels local work and abandons every incomplete multipart upload.
func (s *multipartSession) Abort(ctx context.Context) error {
	s.Cancel()
	resources := s.snapshot()
	var errs []error
	for _, upload := range resources {
		upload.inflight.Wait()
		upload.mu.Lock()
		upload.closed = true
		uploadID, completed := upload.uploadID, upload.completed
		upload.buffer.Release()
		upload.buffer = nil
		upload.mu.Unlock()
		if uploadID == "" || completed {
			continue
		}
		if err := s.withSlot(ctx, func(ctx context.Context) error {
			return s.store.AbortMultipart(ctx, s.bucket, upload.key, uploadID)
		}); err != nil {
			errs = append(errs, fmt.Errorf("s3 sink: abort %s: %w", upload.key, err))
		}
	}
	return errors.Join(errs...)
}
