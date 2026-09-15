// Package streamproof carries process-local pipeline completion evidence. Only
// trusted Filament internals can construct it; it is not a serialized credential.
package streamproof

import (
	"maps"

	"github.com/galaxy-io/filament/rowmodel"
)

// ResourceTotals records completed Apply totals for one destination resource.
type ResourceTotals struct{ Rows, Bytes int64 }

// Completion binds accepted native boundaries and applied totals to one epoch.
// Its zero value is invalid; exported accessors return independently owned data.
type Completion struct {
	binding     string
	positions   rowmodel.DomainPositions
	rows, bytes int64
	resources   map[string]ResourceTotals
}

// New is called by the pipeline only after all covered Apply calls complete.
func New(binding string, positions rowmodel.DomainPositions, rows, nbytes int64, resources map[string]ResourceTotals) Completion {
	return Completion{binding: binding, positions: positions.Clone(), rows: rows, bytes: nbytes, resources: maps.Clone(resources)}
}

// Binding returns the immutable epoch identity.
func (c Completion) Binding() string { return c.binding }

// Positions returns the accepted source boundary positions, independently owned.
func (c Completion) Positions() rowmodel.DomainPositions { return c.positions.Clone() }

// Totals returns completed Apply record and byte totals.
func (c Completion) Totals() (int64, int64) { return c.rows, c.bytes }

// Resources returns independently owned per-resource Apply totals.
func (c Completion) Resources() map[string]ResourceTotals { return maps.Clone(c.resources) }
