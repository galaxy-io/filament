package zerobus

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

// Apply serializes one Arrow batch to Arrow IPC bytes and ingests it into the
// resource's Zerobus stream. The sink advertises only append write policies, so
// the batch's operations are validated against the resource's append policy and
// every accepted row is appended to the Delta table.
func (s *Sink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if opts.Policy.Capability.Mode != filament.WriteAppend {
		return filament.WriteReceipt{}, fmt.Errorf("databrickszerobus sink: write mode %q is not supported (append only)", opts.Policy.Capability.Mode)
	}
	if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("databrickszerobus sink: %w", err)
	}
	stream, ok := s.streams[b.Resource]
	if !ok {
		return filament.WriteReceipt{}, fmt.Errorf("databrickszerobus sink: no schema ensured for resource %q", b.Resource)
	}
	rec := conformRecord(s.schemas[b.Resource], b.Rows())
	defer rec.Release()
	ipc, err := batchIPC(s.schemas[b.Resource], rec)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("databrickszerobus sink: encode %q seq %d: %w", b.Resource, b.Seq, err)
	}
	writeCRC := b.IntegrityCRC()
	if _, err := stream.Ingest(ipc); err != nil {
		s.tel.recordError(ctx, b.Resource, retryable(err))
		return filament.WriteReceipt{}, fmt.Errorf("databrickszerobus sink: ingest %q seq %d: %w", b.Resource, b.Seq, err)
	}
	s.tel.recordBatch(ctx, b.Resource)
	return filament.WriteReceipt{
		URI:      fmt.Sprintf("databricks://%s.%s.%s", s.catalog, s.schema, b.Resource),
		Bytes:    int64(len(ipc)),
		Rows:     b.NumRows(),
		WriteCRC: writeCRC,
	}, nil
}
