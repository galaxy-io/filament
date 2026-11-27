package s3

import (
	"context"
	"io"
)

// completedPart identifies one successfully uploaded multipart chunk. The
// token is interpreted by the provider adapter when completing the upload, and
// crc32c is the part's checksum, which S3 verifies on upload and again when it
// combines parts into the object's composite checksum.
type completedPart struct {
	number int32
	token  string
	crc32c uint32
}

type objectMetadata struct {
	contentType     string
	contentEncoding string
}

// multipartStore is the narrow storage surface used by a multipart session.
// Every write carries the CRC32C of its bytes, and the store must reject a
// write whose stored bytes do not match it: parts by their own checksum, a
// multipart object by its parts' checksums, and a single object by its
// checksum. CompleteMultipart also receives the object's full CRC32C, for a
// store that can check it.
type multipartStore interface {
	CreateMultipart(ctx context.Context, bucket, key string, metadata objectMetadata) (string, error)
	UploadPart(ctx context.Context, bucket, key, uploadID string, number int32, body io.ReadSeeker, size int64, crc32c uint32) (string, error)
	CompleteMultipart(ctx context.Context, bucket, key, uploadID string, parts []completedPart, crc32c uint32) error
	AbortMultipart(ctx context.Context, bucket, key, uploadID string) error
	PutObject(ctx context.Context, bucket, key string, metadata objectMetadata, body io.ReadSeeker, size int64, crc32c uint32) error
	// ListMultipartUploads reports every incomplete multipart upload whose key
	// starts with prefix.
	ListMultipartUploads(ctx context.Context, bucket, prefix string) ([]pendingUpload, error)
}
