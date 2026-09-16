package gcs

import (
	"context"
	"io"
)

type objectMetadata struct {
	contentType     string
	contentEncoding string
}

// resumableStore is the narrow GCS surface used by an upload session.
type resumableStore interface {
	BucketAttrs(context.Context, string) error
	NewWriter(context.Context, string, string, objectMetadata, int) resumableWriter
	Close() error
}

// resumableWriter hides the provider SDK while retaining its server checksum.
type resumableWriter interface {
	io.Writer
	Close() error
	Abort(error)
	CRC32C() uint32
}
