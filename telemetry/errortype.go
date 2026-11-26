package telemetry

import (
	"context"
	"errors"
	"os"
)

// Values of the error.type attribute. Each metric that carries it uses a
// subset, and ErrorTypeOther stands for any failure no value describes.
const (
	ErrorTypeOther          = "_OTHER"
	ErrorTypeCanceled       = "canceled"
	ErrorTypeTimeout        = "timeout"
	ErrorTypeSourceAuth     = "source_auth"
	ErrorTypeSourceConfig   = "source_config"
	ErrorTypeSourceRead     = "source_read"
	ErrorTypeSchemaMismatch = "schema_mismatch"
	ErrorTypeTransform      = "transform"
	ErrorTypeSinkWrite      = "sink_write"
	ErrorTypeWorkerLost     = "worker_lost"
	ErrorTypeInternal       = "internal"

	ErrorTypeAPI         = "api_error"
	ErrorTypeForbidden   = "forbidden"
	ErrorTypeInvalidSpec = "invalid_spec"
)

type typedError struct {
	errorType string
	err       error
}

func (e *typedError) Error() string { return e.err.Error() }

func (e *typedError) Unwrap() error { return e.err }

// WithErrorType marks err with an error.type value, keeping its message. An
// error already marked keeps its first value, so the stage that failed first
// names the failure. A nil err stays nil.
func WithErrorType(errorType string, err error) error {
	if err == nil {
		return nil
	}
	var marked *typedError
	if errors.As(err, &marked) {
		return err
	}
	return &typedError{errorType: errorType, err: err}
}

// ErrorType returns err's error.type value: canceled or timeout when a context
// or deadline ended it, otherwise the value it was marked with, otherwise
// ErrorTypeOther. A nil err has none.
func ErrorType(err error) string {
	if err == nil {
		return ""
	}
	var timeout interface{ Timeout() bool }
	var marked *typedError
	switch {
	case errors.Is(err, context.Canceled):
		return ErrorTypeCanceled
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, os.ErrDeadlineExceeded):
		return ErrorTypeTimeout
	case errors.As(err, &timeout) && timeout.Timeout():
		return ErrorTypeTimeout
	case errors.As(err, &marked):
		return marked.errorType
	default:
		return ErrorTypeOther
	}
}
