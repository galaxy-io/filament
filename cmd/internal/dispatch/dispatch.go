// Package dispatch selects the dispatch backend for requested runs.
package dispatch

import (
	"fmt"
	"os"

	"github.com/galaxy-io/filament/internal/modules/dispatch/inproc"
	"github.com/galaxy-io/filament/internal/modules/dispatch/k8s"
	"github.com/galaxy-io/filament/module"
)

// FromEnv selects the backend per DISPATCH_MODE; kubernetes is the default.
// kubernetes launches one worker Job per run; inproc executes runs inside
// this process.
func FromEnv() (module.Module, error) {
	switch mode := os.Getenv("DISPATCH_MODE"); mode {
	case "", "kubernetes":
		return k8s.NewFromEnv(), nil
	case "inproc":
		return inproc.New(), nil
	default:
		return nil, fmt.Errorf("unknown DISPATCH_MODE %q (kubernetes|inproc)", mode)
	}
}
