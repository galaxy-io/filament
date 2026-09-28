package pipeline

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/internal/streamproof"
)

// BindEpoch binds future Apply calls to a native epoch. Call on the producer
// owner before any writes/controls, then only after sealing the previous epoch.
// It does not call a sink lifecycle method or grant ownership.
func (p *Pipeline) BindEpoch(ref filament.EpochRef) error {
	if err := ref.Validate(); err != nil {
		return err
	}
	in := p.stream
	if in == nil {
		return filament.ErrEpochMismatch
	}
	if err := in.ready(); err != nil {
		return err
	}
	if in.epoch == nil {
		if in.touched {
			return filament.ErrEpochMismatch
		}
	} else if !in.sealed || ref.Attempt != in.epoch.Attempt || ref.Epoch != in.epoch.Epoch+1 {
		return filament.ErrEpochMismatch
	}
	in.epoch = &ref
	in.sealed = false
	in.completed = make(filament.DomainPositions)
	in.epochRows, in.epochBytes = 0, 0
	in.epochResources = make(map[string]streamproof.ResourceTotals)
	return nil
}

// SealEpoch returns immutable completion evidence and prevents further writes
// until BindEpoch. Every covered domain must have a safe accepted control after
// the last row; controls are deliberately conservative in this single-lane slice.
// Sink commits and source-prefix correctness remain the coordinator's and native
// connector's responsibilities. No evidence is serialized as authority.
func (p *Pipeline) SealEpoch() (filament.EpochCompletion, error) {
	in := p.stream
	if in == nil {
		return filament.EpochCompletion{}, filament.ErrEpochMismatch
	}
	if err := in.ready(); err != nil {
		return filament.EpochCompletion{}, err
	}
	if in.epoch == nil || in.sealed || len(in.transactions) != 0 || len(in.completed) == 0 {
		return filament.EpochCompletion{}, filament.ErrIncompleteCoverage
	}
	in.sealed = true
	return streamproof.New(in.epoch.Binding(), in.completed, in.epochRows, in.epochBytes, in.epochResources), nil
}
