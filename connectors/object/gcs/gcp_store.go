package gcs

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
)

type gcpStore struct{ client *storage.Client }

var _ resumableStore = (*gcpStore)(nil)

func newGCPStore(ctx context.Context) (*gcpStore, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcs sink: create client: %w", err)
	}
	return &gcpStore{client: client}, nil
}

func (s *gcpStore) BucketAttrs(ctx context.Context, bucket string) error {
	_, err := s.client.Bucket(bucket).Attrs(ctx)
	return err
}

func (s *gcpStore) NewWriter(
	ctx context.Context,
	bucket string,
	key string,
	metadata objectMetadata,
	chunkSize int,
) resumableWriter {
	writerCtx, cancel := context.WithCancelCause(ctx)
	writer := s.client.Bucket(bucket).Object(key).NewWriter(writerCtx)
	writer.ChunkSize = chunkSize
	writer.ContentType = metadata.contentType
	writer.ContentEncoding = metadata.contentEncoding
	return &gcpWriter{writer: writer, cancel: cancel}
}

func (s *gcpStore) Close() error { return s.client.Close() }

type gcpWriter struct {
	writer *storage.Writer
	cancel context.CancelCauseFunc
}

var _ resumableWriter = (*gcpWriter)(nil)

func (w *gcpWriter) Write(data []byte) (int, error) { return w.writer.Write(data) }

func (w *gcpWriter) Close() error {
	err := w.writer.Close()
	w.cancel(err)
	return err
}

func (w *gcpWriter) Abort(err error) { w.cancel(err) }

func (w *gcpWriter) CRC32C() uint32 { return w.writer.Attrs().CRC32C }
