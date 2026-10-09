package streamkit

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
)

// Layout returns the decoder state persisted with an epoch's progress.
func (c *MessageColumns) Layout() schema.MessageLayout {
	return schema.MessageLayout{Raw: c.raw, Fields: slices.Clone(c.fields)}
}

// RestoreMessageColumns restores the accepted payload layout without sampling.
func RestoreMessageColumns(base rowmodel.Schema, layout schema.MessageLayout) (*MessageColumns, rowmodel.Schema, error) {
	if err := layout.Validate(); err != nil {
		return nil, rowmodel.Schema{}, err
	}
	occupied := map[string]bool{}
	for _, f := range base.Fields {
		occupied[strings.ToLower(f.Name)] = true
	}
	for _, f := range layout.Fields {
		if occupied[strings.ToLower(f.Name)] {
			return nil, rowmodel.Schema{}, fmt.Errorf("message field %q collides with transport metadata", f.Name)
		}
	}
	base = base.Clone()
	base.Fields = append(base.Fields, layout.Fields...)
	model, err := rowmodel.WithEventMetadataFields(base)
	return &MessageColumns{resource: base.Resource, raw: layout.Raw, fields: slices.Clone(layout.Fields)}, model, err
}

// ProbeMessageBatch inspects every buffered message before any row is encoded.
// It keeps absent fields and adds observed fields in deterministic order. An
// initial batch establishes a layout; later batches obey the configured policy.
func ProbeMessageBatch(base rowmodel.Schema, accepted *schema.MessageLayout, payloads [][]byte, policy schema.Policy) (*MessageColumns, rowmodel.Schema, error) {
	return probeMessageBatch(context.Background(), base, accepted, payloads, policy)
}

func probeMessageBatch(ctx context.Context, base rowmodel.Schema, accepted *schema.MessageLayout, payloads [][]byte, policy schema.Policy) (*MessageColumns, rowmodel.Schema, error) {
	if err := policy.Validate(); err != nil {
		return nil, rowmodel.Schema{}, err
	}
	if len(payloads) == 0 {
		return nil, rowmodel.Schema{}, fmt.Errorf("cannot infer an empty message batch")
	}
	var columns *MessageColumns
	var err error
	if accepted == nil {
		columns, _, err = NewMessageColumns(base, payloads[0])
	} else {
		columns, _, err = RestoreMessageColumns(base, *accepted)
	}
	if err != nil {
		return nil, rowmodel.Schema{}, &schema.BlockError{Resource: base.Resource, Reasons: []string{err.Error()}}
	}
	candidate := columns.Layout()
	known := map[string]bool{}
	for _, f := range candidate.Fields {
		known[f.Name] = true
	}
	additions := map[string]rowmodel.Field{}
	for _, payload := range payloads {
		values, object, e := decodeObject(payload)
		if e != nil {
			return nil, rowmodel.Schema{}, &schema.BlockError{Resource: base.Resource, Reasons: []string{e.Error()}}
		}
		if object && !candidate.Raw {
			for name, value := range values {
				if known[name] {
					continue
				}
				f := rowmodel.Field{Name: name, Logical: messageType(value), Nullable: true}
				if previous, ok := additions[name]; ok && previous.Logical != f.Logical {
					// JSON accommodates null-only observations and heterogeneous
					// initial values without coercing them to strings.
					f.Logical = rowmodel.LogicalJSON
				}
				additions[name] = f
			}
		}
	}
	names := make([]string, 0, len(additions))
	for name := range additions {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		candidate.Fields = append(candidate.Fields, additions[name])
	}
	columns, model, err := RestoreMessageColumns(base, candidate)
	if err != nil {
		return nil, rowmodel.Schema{}, &schema.BlockError{Resource: base.Resource, Reasons: []string{err.Error()}}
	}
	if accepted != nil {
		diff, err := schema.Compare(rowmodel.Schema{Resource: base.Resource, Fields: accepted.Fields}, rowmodel.Schema{Resource: base.Resource, Fields: candidate.Fields})
		if err != nil {
			return nil, rowmodel.Schema{}, err
		}
		schema.Observe(ctx, "source", base.Resource, policy, diff)
		if err := schema.CheckMessageChange(base.Resource, *accepted, candidate, policy); err != nil {
			return nil, rowmodel.Schema{}, err
		}
	}
	for _, payload := range payloads {
		if _, err := columns.Prepare(payload); err != nil {
			return nil, rowmodel.Schema{}, err
		}
	}
	return columns, model, nil
}
