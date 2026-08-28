package s3

import (
	"context"
	"io"
)

// completedPart identifies one successfully uploaded multipart chunk. The
// token is interpreted by the provider adapter when completing the upload.
type completedPart struct {
	number int32
	token  string
}

// multipartStore is the narrow storage surface used by a multipart session.
type multipartStore interface {
	CreateMultipart(context.Context, string, string, string) (string, error)
	UploadPart(context.Context, string, string, string, int32, io.ReadSeeker, int64) (string, error)
	CompleteMultipart(context.Context, string, string, string, []completedPart) error
	AbortMultipart(context.Context, string, string, string) error
	PutObject(context.Context, string, string, string, io.ReadSeeker, int64) error
}
