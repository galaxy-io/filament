package clickhouse

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// Apply validates the requested write policy and inserts one typed batch.
func (s *Sink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if want := s.modeFor(b.Resource); opts.Policy.Capability.Mode != want {
		return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: apply policy %q does not match resource policy %q", opts.Policy.Capability.Mode, want)
	}
	switch opts.Policy.Capability.Mode {
	case filament.WriteReplace:
		policy := opts.Policy
		policy.Capability.AcceptsOps = []rowmodel.Operation{rowmodel.OpInsert}
		if err := policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: %w", err)
		}
	case filament.WriteAppend:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: %w", err)
		}
	case filament.WriteUpsert:
		policy := opts.Policy
		policy.Capability.AcceptsOps = []rowmodel.Operation{rowmodel.OpInsert, rowmodel.OpUpdate}
		if err := policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: %w", err)
		}
	default:
		return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: write policy %q is not implemented", opts.Policy.Capability.Mode)
	}
	return s.write(ctx, b)
}

func (s *Sink) write(ctx context.Context, b *arrowbatch.Batch) (filament.WriteReceipt, error) {
	if s.conn == nil {
		return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: write before open")
	}
	tbl := s.tables[b.Resource]
	if tbl == nil {
		return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: no schema ensured for resource %q", b.Resource)
	}
	batchIn, err := s.conn.PrepareBatch(ctx, tbl.insertSQL)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: prepare %s seq %d: %w", b.Resource, b.Seq, err)
	}
	defer func() { _ = batchIn.Close() }()

	rows := b.Rows()
	cols := rows.Columns()
	fns := make([]valueFn, len(cols))
	for i, f := range rows.Schema().Fields() {
		fns[i] = valueFor(f)
	}
	values := make([]any, len(cols))
	for i := range b.NumRows() {
		for c, col := range cols {
			if col.IsNull(i) {
				values[c] = nil
				continue
			}
			values[c] = fns[c](col, i)
		}
		if err := batchIn.Append(values...); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: append %s seq %d row %d: %w", b.Resource, b.Seq, i, err)
		}
	}
	writeCRC := b.IntegrityCRC()
	if err := batchIn.Send(); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: send %s seq %d: %w", b.Resource, b.Seq, err)
	}
	s.written.Add(int64(b.NumRows()))
	return filament.WriteReceipt{
		URI: fmt.Sprintf("clickhouse://%s.%s", s.database, b.Resource), Bytes: arrowbatch.Bytes(rows),
		Rows: b.NumRows(), WriteCRC: writeCRC,
	}, nil
}
