// Package inproc is the in-process dispatch backend: it mounts the engine
// module so requested runs execute inside the calling process.
package inproc

import (
	"github.com/galaxy-io/filament/internal/modules/engine"
	"github.com/galaxy-io/filament/module"
)

// New returns the in-process dispatch backend.
func New() module.Module { return engine.New() }
