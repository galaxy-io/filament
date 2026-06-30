// Package scenarios wires the built-in seed scenarios (small, large) to their
// backend seeders. Import it blank to activate:
//
//	import _ "github.com/galaxy-io/filament/tests/testcontainers/seed/scenarios"
package scenarios

import (
	"github.com/galaxy-io/filament/tests/testcontainers/seed"
	pgseeder "github.com/galaxy-io/filament/tests/testcontainers/seed/postgres"
)

func init() {
	seed.AddSeeder("multitenant-sm", "postgres", pgseeder.Seeder(seed.Small))
	seed.AddSeeder("multitenant-lg", "postgres", pgseeder.Seeder(seed.Large))
	seed.AddDropper("multitenant-sm", "postgres", pgseeder.Dropper(seed.Small))
	seed.AddDropper("multitenant-lg", "postgres", pgseeder.Dropper(seed.Large))
	seed.AddVerifier("multitenant-sm", "postgres", pgseeder.Verifier(seed.Small))
	seed.AddVerifier("multitenant-lg", "postgres", pgseeder.Verifier(seed.Large))
}
