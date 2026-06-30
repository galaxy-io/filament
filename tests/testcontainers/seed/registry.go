package seed

import (
	"fmt"
	"sort"
	"sync"
)

// Scenario is a named, describable seed configuration. Register scenarios
// with Register() so the gx CLI and tests can refer to them by name.
// Seeders maps DSN scheme (e.g. "postgres", "redis") to the function that
// seeds that backend — the gx CLI dispatches based on the --target scheme.
type Scenario struct {
	Name        string
	Description string
	Spec        Spec
	Seeders     map[string]SeederFunc
	Droppers    map[string]DropFunc
	Verifiers   map[string]VerifyFunc
}

var (
	mu        sync.RWMutex
	scenarios = map[string]Scenario{}
)

// Register adds a Scenario to the global registry. Call from init() in a
// seed driver package or in test setup. Panics on duplicate names.
func Register(s Scenario) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := scenarios[s.Name]; exists {
		panic(fmt.Sprintf("seed: scenario %q already registered", s.Name))
	}
	scenarios[s.Name] = s
}

// Get returns the named Scenario. ok is false if the name is not registered.
func Get(name string) (Scenario, bool) {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := scenarios[name]
	return s, ok
}

// List returns all registered scenarios sorted by name.
func List() []Scenario {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Scenario, 0, len(scenarios))
	for _, s := range scenarios {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// AddSeeder attaches a SeederFunc for the given scheme to an already-registered
// Scenario. Call from an init() in a driver package to wire seeders without
// creating an import cycle (seed → seed/postgres would be circular).
func AddSeeder(name, scheme string, fn SeederFunc) {
	mu.Lock()
	defer mu.Unlock()
	s := mustGet(name, "AddSeeder")
	if s.Seeders == nil {
		s.Seeders = map[string]SeederFunc{}
	}
	s.Seeders[scheme] = fn
	scenarios[name] = s
}

// AddDropper attaches a DropFunc for the given scheme to an already-registered
// Scenario.
func AddDropper(name, scheme string, fn DropFunc) {
	mu.Lock()
	defer mu.Unlock()
	s := mustGet(name, "AddDropper")
	if s.Droppers == nil {
		s.Droppers = map[string]DropFunc{}
	}
	s.Droppers[scheme] = fn
	scenarios[name] = s
}

// AddVerifier attaches a VerifyFunc for the given scheme to an already-registered
// Scenario.
func AddVerifier(name, scheme string, fn VerifyFunc) {
	mu.Lock()
	defer mu.Unlock()
	s := mustGet(name, "AddVerifier")
	if s.Verifiers == nil {
		s.Verifiers = map[string]VerifyFunc{}
	}
	s.Verifiers[scheme] = fn
	scenarios[name] = s
}

func mustGet(name, caller string) Scenario {
	s, ok := scenarios[name]
	if !ok {
		panic(fmt.Sprintf("seed: %s: scenario %q not registered", caller, name))
	}
	return s
}

func init() {
	Register(Scenario{
		Name:        "multitenant-sm",
		Description: "multi-tenant events: 2 tables × 1 000 rows — fast integration baseline",
		Spec:        Small,
	})
	Register(Scenario{
		Name:        "multitenant-lg",
		Description: "multi-tenant events: 10 tables × 100 000 rows — volume / bench target",
		Spec:        Large,
	})
}
