package webhook

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
	"github.com/galaxy-io/filament/webhooks/stream"
)

// OpenStream writes each delivery as an envelope row with its raw body.
func (s *Source) OpenStream(ctx context.Context, opts filament.StreamOpenOpts) (filament.StreamSession, error) {
	if opts.MessageSchemaPolicy != "" {
		return nil, errors.New("webhook: use OpenMessageStream for schema negotiation")
	}
	writers := map[string]arrowbatch.RowWriter{}
	return s.open(ctx, opts, func(_ context.Context, out filament.StreamRecordSink, batch []stream.Delivery) error {
		return writeEnvelopes(out, writers, batch)
	})
}

// OpenMessageStream projects JSON bodies into columns. It probes each batch
// before emitting rows and restores the accepted layout from the previous
// certified epoch.
func (s *Source) OpenMessageStream(ctx context.Context, opts filament.StreamOpenOpts) (filament.StreamSession, error) {
	live, err := streamkit.NewMessageBatches(opts, messageBaseSchema)
	if err != nil {
		return nil, err
	}
	if live == nil {
		return nil, errors.New("webhook: message schema policy required")
	}
	return s.open(ctx, opts, func(ctx context.Context, out filament.StreamRecordSink, batch []stream.Delivery) error {
		return writeNegotiated(ctx, out, live, s, batch)
	})
}

func (s *Source) open(ctx context.Context, opts filament.StreamOpenOpts, write stream.WriteFunc) (filament.StreamSession, error) {
	if s.inbox == nil {
		return nil, errors.New("webhook: source is not configured")
	}
	return stream.Open(ctx, stream.Config{Inbox: s.inbox, Batch: s.batch, Codecs: s, Write: write}, opts)
}

// writeEnvelopes emits each delivery with its raw body in a payload column.
func writeEnvelopes(out filament.StreamRecordSink, writers map[string]arrowbatch.RowWriter, batch []stream.Delivery) error {
	for _, d := range batch {
		writer := writers[d.Resource]
		if writer == nil {
			var err error
			if writer, err = out.Builder(d.Resource, 0, envelopeSchema(d.Resource)); err != nil {
				return err
			}
			writers[d.Resource] = writer
		}
		metadata, err := deliveryMetadata(d)
		if err != nil {
			return err
		}
		writer.String(d.ID)
		writer.String(d.ProviderDeliveryID)
		writer.Timestamp(d.AcceptedAt.UnixMicro())
		writer.Bytes(d.Body)
		writer.Bytes(metadata)
		if err := writer.EndRow(rowmodel.Meta{Stream: &rowmodel.StreamMeta{Identity: d.Identity}}); err != nil {
			return err
		}
	}
	return nil
}

// writeNegotiated negotiates every resource's layout for the batch before
// writing any row, then projects each JSON body into columns.
func writeNegotiated(ctx context.Context, out filament.StreamRecordSink, live *streamkit.MessageBatches,
	codecs rowmodel.CodecResolver, batch []stream.Delivery,
) error {
	payloads := map[string][][]byte{}
	for _, d := range batch {
		payloads[d.Resource] = append(payloads[d.Resource], d.Body)
	}
	columns := map[string]*streamkit.MessageColumns{}
	writers := map[string]arrowbatch.RowWriter{}
	projectors := map[string]*streamkit.Projector{}
	for resource, bodies := range payloads {
		column, writer, err := live.Prepare(ctx, out, messageBaseSchema(resource), bodies)
		if err != nil {
			return err
		}
		columns[resource], writers[resource] = column, writer
		projectors[resource] = streamkit.NewEventMetadataProjector(writer, codecs)
	}
	for _, d := range batch {
		writer := writers[d.Resource]
		metadata, err := deliveryMetadata(d)
		if err != nil {
			return err
		}
		writer.String(d.ID)
		writer.String(d.ProviderDeliveryID)
		writer.Timestamp(d.AcceptedAt.UnixMicro())
		writer.Bytes(metadata)
		writer.String(d.Tenant)
		writer.String(d.Endpoint)
		appendPayload, err := columns[d.Resource].Prepare(d.Body)
		if err != nil {
			return err
		}
		appendPayload(writer)
		if err := projectors[d.Resource].EndEvent(streamkit.Envelope{
			Identity: d.Identity, Timestamp: &d.AcceptedAt, KeyNull: true, Payload: d.Body,
		}, rowmodel.Meta{}); err != nil {
			return err
		}
	}
	return nil
}

func deliveryMetadata(d stream.Delivery) ([]byte, error) {
	return json.Marshal(map[string]string{"resource": d.Resource})
}
