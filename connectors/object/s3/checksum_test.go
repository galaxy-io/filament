package s3

import (
	"context"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/galaxy-io/filament"
)

// corruptStore flips one byte of the first body it receives for an operation,
// standing in for a transfer or storage fault the checksum must catch. Part
// uploads run concurrently, so each one-shot flag is claimed atomically.
type corruptStore struct {
	*limitStore
	part, put atomic.Bool
}

type flipReader struct {
	io.ReadSeeker
	done bool
}

func (f *flipReader) Read(p []byte) (int, error) {
	n, err := f.ReadSeeker.Read(p)
	if n > 0 && !f.done {
		p[0] ^= 0xff
		f.done = true
	}
	return n, err
}

func (s *corruptStore) UploadPart(ctx context.Context, bucket, key, uploadID string, number int32, body io.ReadSeeker, size int64, crc uint32) (string, error) {
	if s.part.CompareAndSwap(true, false) {
		body = &flipReader{ReadSeeker: body}
	}
	return s.limitStore.UploadPart(ctx, bucket, key, uploadID, number, body, size, crc)
}

func (s *corruptStore) PutObject(ctx context.Context, bucket, key string, metadata objectMetadata, body io.ReadSeeker, size int64, crc uint32) error {
	if s.put.CompareAndSwap(true, false) {
		body = &flipReader{ReadSeeker: body}
	}
	return s.limitStore.PutObject(ctx, bucket, key, metadata, body, size, crc)
}

func TestCorruptedWriteFailsRun(t *testing.T) {
	for _, test := range []struct {
		name      string
		store     func() *corruptStore
		writes    int
		wantError string
	}{
		{"multipart part", func() *corruptStore {
			store := &corruptStore{limitStore: newLimitStore()}
			store.part.Store(true)
			return store
		}, 8, "CRC32C mismatch"},
		{"single object", func() *corruptStore {
			store := &corruptStore{limitStore: newLimitStore()}
			store.put.Store(true)
			return store
		}, 1, "CRC32C mismatch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			store := test.store()
			sink, policy := openTestSink(t, ctx, store, "ndjson", "none", rollRun())
			batch := testBatch(t, 0)
			defer batch.Release()
			var err error
			for range test.writes {
				if _, err = sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policy}); err != nil {
					break
				}
			}
			if err == nil {
				err = sink.Commit(ctx)
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("run error = %v, want %q", err, test.wantError)
			}
			if _, ok := store.object(sink.layout.Success()); ok {
				t.Fatal("success manifest published after a corrupted write")
			}
			_ = sink.Abort(ctx)
		})
	}
}
