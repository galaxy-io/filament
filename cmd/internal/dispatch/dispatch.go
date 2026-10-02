// Package dispatch selects the dispatch backend for requested runs.
package dispatch

import (
	"fmt"
	"os"

	"github.com/galaxy-io/filament/internal/modules/dispatch/k8s"
	"github.com/galaxy-io/filament/module"
)

// FromEnv selects the backend per DISPATCH_MODE. kubernetes, the default,
// launches one worker Job per run. worker returns no backend: a long-lived
// worker started with -execute consumes requested runs itself.
func FromEnv() (module.Module, error) {
	switch mode := os.Getenv("DISPATCH_MODE"); mode {
	case "", "kubernetes":
		return k8s.NewFromEnv(), nil
	case "worker":
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown DISPATCH_MODE %q (kubernetes|worker)", mode)
	}
}
