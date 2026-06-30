package secret

import "errors"

// ErrNotFound is returned when a referenced secret does not exist.
var ErrNotFound = errors.New("secret not found")

// ErrReadOnly is returned by providers that do not support writes.
var ErrReadOnly = errors.New("secret provider is read-only")
