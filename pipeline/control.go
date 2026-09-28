package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/internal/streamproof"
	"github.com/galaxy-io/filament/rowmodel"
)

// NewStream constructs an opt-in, single-writer pipeline. Records implements
// filament.StreamRecordSink. All producer operations, including Barrier, must be
// serialized on the builder-owning goroutine. Global strict ordering flushes on
// every writer switch; transaction ordering may batch across resources until a
// control. Partition scheduling is not supported. This does not enable the runner
// or invoke a destination's epoch lifecycle.
func NewStream(cfg Config, ordering filament.Ordering) (*Pipeline, error) {
	switch ordering {
	case filament.OrderingNone, filament.OrderingTransaction, filament.OrderingGlobalStrict:
	default:
		return nil, fmt.Errorf("pipeline: unsupported serial stream ordering %q", ordering)
	}
	cfg.Options.SnapshotParallelism = 1
	p := New(cfg)
	p.stream = &streamInlet{p: p, strict: ordering == filament.OrderingGlobalStrict, transactions: make(map[rowmodel.DomainKey]string)}
	return p, nil
}

type streamInlet struct {
	p                     *Pipeline
	strict                bool
	writers               []*streamWriter
	active                *streamWriter
	transactions          map[rowmodel.DomainKey]string
	sequence              uint64
	closed                bool
	epoch                 *filament.EpochRef
	touched, sealed       bool
	completed             filament.DomainPositions
	epochRows, epochBytes int64
	epochResources        map[string]streamproof.ResourceTotals
}

var _ filament.StreamRecordSink = (*streamInlet)(nil)

func (in *streamInlet) Builder(resource string, part int, schema rowmodel.Schema) (arrowbatch.RowWriter, error) {
	if err := in.ready(); err != nil {
		return nil, err
	}
	for _, w := range in.writers {
		if w.resource == resource {
			return nil, errors.New("pipeline: serial stream permits one builder per resource")
		}
	}
	w, err := (&inlet{p: in.p}).Builder(resource, part, schema)
	if err != nil {
		return nil, err
	}
	owned := &streamWriter{RowWriter: w, in: in, resource: resource}
	in.writers = append(in.writers, owned)
	return owned, nil
}

func (in *streamInlet) Control(ctx context.Context, c rowmodel.Control) error {
	_, err := in.p.Barrier(ctx, c)
	return err
}

func (in *streamInlet) ready() error {
	if err := in.p.Err(); err != nil {
		return err
	}
	if in.closed {
		return ErrPipelineClosed
	}
	select {
	case <-in.p.done:
		return in.failure()
	default:
		return nil
	}
}

func (in *streamInlet) failure() error {
	if err := in.p.Err(); err != nil {
		return err
	}
	return ErrPipelineClosed
}

func (in *streamInlet) fail(err error) error { in.p.setErr(err); return in.p.Err() }

func (in *streamInlet) flush() error {
	for _, w := range in.writers {
		if !w.closed {
			if err := w.Flush(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (in *streamInlet) validateControl(c rowmodel.Control) error {
	if err := c.Validate(); err != nil {
		return err
	}
	txn := in.transactions[c.Domain]
	switch c.Kind {
	case rowmodel.TxnBegin:
		if txn != "" {
			return errors.New("pipeline: transaction already open")
		}
	case rowmodel.TxnEnd:
		if txn != c.TxnID {
			return errors.New("pipeline: transaction end does not match begin")
		}
	case rowmodel.ProgressBoundary, rowmodel.MemberActivated:
		if txn != "" {
			return errors.New("pipeline: progress control inside transaction")
		}
	}
	return nil
}

func (in *streamInlet) acceptControl(c rowmodel.Control) {
	switch c.Kind {
	case rowmodel.TxnBegin:
		in.transactions[c.Domain] = c.TxnID
	case rowmodel.TxnEnd:
		delete(in.transactions, c.Domain)
	}
}
