//go:build integration || e2e

package testutil

import (
	"github.com/galaxy-io/filament/tests/testcontainers/seed"
	"github.com/galaxy-io/filament/tests/testcontainers/seed/tpch"
)

// RegisterTPCHSmokeScenario makes the small e2e dataset available once per
// test process.
func RegisterTPCHSmokeScenario() {
	if _, ok := seed.Get("tpch-sf0.01"); !ok {
		tpch.RegisterSF(0.01)
	}
}
