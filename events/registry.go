package events

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/galaxy-io/filament/eventbus"
)

// Definition is one registry entry: the subject coordinates and payload
// decoder of a defined event kind.
type Definition struct {
	Entity string
	Event  string
	decode func([]byte) (any, error)
}

// Name returns the wire spelling "<entity>.<event>".
func (d Definition) Name() string { return d.Entity + eventbus.Separator + d.Event }

var (
	regMu    sync.RWMutex
	registry = map[string]Definition{}
)

// define registers an event kind under its wire spelling "<entity>.<event>"
// (e.g. "batch.written") and returns its EventType. A malformed or duplicate
// name panics: the catalog is a compile-time artifact and collisions are bugs.
func define[T any](name string) EventType[T] {
	entity, event, ok := strings.Cut(name, eventbus.Separator)
	if !ok || entity == "" || event == "" || strings.Contains(event, eventbus.Separator) {
		panic(fmt.Sprintf("events: %q is not <entity>.<event>", name))
	}
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("events: %q defined twice", name))
	}
	registry[name] = Definition{
		Entity: entity,
		Event:  event,
		decode: func(b []byte) (any, error) {
			var v T
			if len(b) > 0 {
				if err := json.Unmarshal(b, &v); err != nil {
					return nil, err
				}
			}
			return v, nil
		},
	}
	return EventType[T]{entity: entity, name: event}
}

// Lookup returns the Definition registered under the wire spelling.
func Lookup(name string) (Definition, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	d, ok := registry[name]
	return d, ok
}

// Names returns every registered wire spelling, sorted.
func Names() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}
