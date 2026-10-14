package streamkit

import (
	"context"
	"errors"
	"slices"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
)

// MessageBatches owns accepted layouts for one source session. Share it across
// physical readers of the same logical resources. Only the serial Read owner
// may call Prepare. A failed epoch ends the session and discards tentative state.
type MessageBatches struct {
	policy  schema.Policy
	layouts map[string]schema.MessageLayout
}

// NewMessageBatches validates and restores layouts. A nil result means disabled.
// base supplies transport columns, without payload or Filament metadata fields.
func NewMessageBatches(opts filament.StreamOpenOpts, base func(string) rowmodel.Schema) (*MessageBatches, error) {
	if opts.MessageSchemaPolicy == "" {
		if opts.MessageSchemas != nil {
			return nil, &schema.BlockError{Reasons: []string{"cannot discard accepted message layouts"}}
		}
		return nil, nil
	}
	if err := opts.MessageSchemaPolicy.Validate(); err != nil {
		return nil, err
	}
	if len(opts.CommittedPositions) > 0 && len(opts.MessageSchemas) == 0 {
		return nil, &schema.BlockError{Reasons: []string{"committed input has no accepted message layouts"}}
	}
	layouts := schema.CloneMessageLayouts(opts.MessageSchemas)
	if layouts == nil {
		layouts = map[string]schema.MessageLayout{}
	}
	for resource, layout := range layouts {
		if !slices.Contains(opts.Resources, resource) {
			return nil, filament.ErrMembershipChanged
		}
		if _, _, err := RestoreMessageColumns(base(resource), layout); err != nil {
			return nil, err
		}
	}
	return &MessageBatches{policy: opts.MessageSchemaPolicy, layouts: layouts}, nil
}

// Prepare validates every payload, restores the decoder and negotiates a writer
// before any row is emitted. Transport fields and EndRow remain source-owned.
// Successful admission is tentative until the runner certifies the epoch.
func (b *MessageBatches) Prepare(ctx context.Context, out filament.StreamRecordSink, base rowmodel.Schema, payloads [][]byte) (*MessageColumns, filament.RowWriter, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	records, ok := out.(filament.MessageSchemaRecordSink)
	if !ok {
		return nil, nil, errors.New("message schema negotiation unavailable")
	}
	var accepted *schema.MessageLayout
	if previous, ok := b.layouts[base.Resource]; ok {
		accepted = &previous
	}
	columns, model, err := probeMessageBatch(ctx, base, accepted, payloads, b.policy)
	if err != nil {
		return nil, nil, err
	}
	layout := columns.Layout()

	writer, err := records.MessageBuilder(ctx, base.Resource, model, layout)
	if err != nil {
		return nil, nil, err
	}
	b.layouts[base.Resource] = layout.Clone()
	return columns, writer, nil
}
