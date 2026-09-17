package arrowbatch

import "github.com/galaxy-io/filament/rowmodel"

// NewControl validates and clones a rowless stream control. The returned batch
// owns its position bytes and follows the same handoff rules as a data batch.
func NewControl(control rowmodel.Control) (*Batch, error) {
	if err := control.Validate(); err != nil {
		return nil, err
	}
	owned := control.Clone()
	b := &Batch{Control: &owned}
	b.refs.Store(1)
	return b, nil
}
