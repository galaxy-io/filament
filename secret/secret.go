// Package secret defines the secret-provider interface. Providers (env, Doppler,
// GCSM, ...) live in subpackages and implement it; a module imports the one it
// needs. Secret values never touch the bus and are never logged.
package secret

import "context"

// Provider reads and writes secrets by reference.
type Provider interface {
	Read(ctx context.Context, ref string) (Value, error)
	Write(ctx context.Context, ref string, v Value) error
	Delete(ctx context.Context, ref string) error
	Name() string
}

// Value is a resolved secret, held in memory only.
type Value struct {
	Bytes []byte
	Meta  map[string]string
}
