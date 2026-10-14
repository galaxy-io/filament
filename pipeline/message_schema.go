package pipeline

import (
	"context"
	"fmt"
	"slices"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
	schemadrift "github.com/galaxy-io/filament/schema"
)

var _ filament.MessageSchemaRecordSink = (*streamInlet)(nil)

// MessageBuilder replaces only drained builders, before any rows in the new
// epoch. It runs on the producer owner. The next batch invokes PrepareSchema on
// the writer before Apply, inside the sink's already-open epoch transaction.
func (in *streamInlet) MessageBuilder(ctx context.Context, resource string, model rowmodel.Schema, layout schemadrift.MessageLayout) (filament.RowWriter, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := in.ready(); err != nil {
		return nil, err
	}
	if in.epoch == nil || in.sealed || in.epochWritten || len(in.transactions) > 0 {
		return nil, filament.ErrEpochMismatch
	}
	if err := in.p.messagePolicy.Validate(); err != nil {
		return nil, err
	}
	if err := layout.Validate(); err != nil {
		return nil, err
	}
	if model.Resource != resource {
		return nil, fmt.Errorf("message schema resource differs")
	}
	if err := rowmodel.ValidateReservedFields(model); err != nil {
		return nil, err
	}
	if err := schemadrift.Validate(model); err != nil {
		return nil, err
	}
	for _, f := range layout.Fields {
		if !slices.Contains(model.Fields, f) {
			return nil, fmt.Errorf("message layout differs from builder model")
		}
	}
	if previous, ok := in.p.messageSchemas[resource]; ok {
		if err := schemadrift.CheckMessageChange(resource, previous, layout, in.p.messagePolicy); err != nil {
			return nil, err
		}
	}
	in.p.registryMu.Lock()
	registered, exists := in.p.schemas[resource]
	in.p.registryMu.Unlock()
	for i, w := range in.writers {
		if w.resource != resource {
			continue
		}
		if exists && registered.model.Equal(model) {
			if in.p.messageSchemas == nil {
				in.p.messageSchemas = map[string]schemadrift.MessageLayout{}
			}
			in.p.messageSchemas[resource] = layout.Clone()
			return w, nil
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		in.writers = slices.Delete(in.writers, i, i+1)
		in.p.registryMu.Lock()
		delete(in.p.schemas, resource)
		for key := range in.p.builders {
			if key.resource == resource {
				delete(in.p.builders, key)
			}
		}
		in.p.registryMu.Unlock()
		break
	}
	w, err := in.Builder(resource, 0, model)
	if err != nil {
		return nil, err
	}
	if in.p.messageSchemas == nil {
		in.p.messageSchemas = map[string]schemadrift.MessageLayout{}
	}
	in.p.messageSchemas[resource] = layout.Clone()
	return w, nil
}
