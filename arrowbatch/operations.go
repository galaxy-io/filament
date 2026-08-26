package arrowbatch

import (
	"slices"

	"github.com/galaxy-io/filament/rowmodel"
)

// Operations is an immutable row-operation vector. An empty vector means every
// row is an insert. Its backing slice is never exposed, preventing sinks from
// mutating batch semantics through a borrowed reference.
type Operations struct {
	values []rowmodel.Operation
}

// NewOperations copies values into an immutable operation vector.
func NewOperations(values []rowmodel.Operation) Operations {
	return Operations{values: slices.Clone(values)}
}

// takeOperations transfers package-owned values without copying. The caller
// must discard its slice after calling this function.
func takeOperations(values []rowmodel.Operation) Operations {
	return Operations{values: values}
}

// Len returns the number of explicitly stored operations. Zero means inserts.
func (o Operations) Len() int { return len(o.values) }

// At returns the explicitly stored operation at i.
func (o Operations) At(i int) rowmodel.Operation { return o.values[i] }

// Clone returns a caller-owned copy for storage outside the batch lifetime.
func (o Operations) Clone() []rowmodel.Operation { return slices.Clone(o.values) }
