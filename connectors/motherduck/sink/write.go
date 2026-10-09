package motherduck

import (
	"context"
	"database/sql/driver"
	"fmt"

	duckdb "github.com/marcboeker/go-duckdb/v2"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// Apply validates the requested write policy and appends one Arrow batch.
func (s *Sink) Apply(ctx context.Context, batch *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if batch == nil {
		return filament.WriteReceipt{}, fmt.Errorf("%s sink: write requires a record batch", s.Name())
	}
	if want := s.modeFor(batch.Resource); opts.Policy.Capability.Mode != want {
		return filament.WriteReceipt{}, fmt.Errorf("%s sink: apply policy %q does not match resource policy %q", s.Name(), opts.Policy.Capability.Mode, want)
	}
	switch opts.Policy.Capability.Mode {
	case filament.WriteReplace:
		policy := opts.Policy
		policy.Capability.AcceptsOps = []rowmodel.Operation{rowmodel.OpInsert}
		if err := policy.ValidateBatch(batch.Resource, batch); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("%s sink: %w", s.Name(), err)
		}
	case filament.WriteAppend:
		if err := opts.Policy.ValidateBatch(batch.Resource, batch); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("%s sink: %w", s.Name(), err)
		}
	case filament.WriteUpsert:
		policy := opts.Policy
		policy.Capability.AcceptsOps = []rowmodel.Operation{rowmodel.OpInsert, rowmodel.OpUpdate}
		if err := policy.ValidateBatch(batch.Resource, batch); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("%s sink: %w", s.Name(), err)
		}
	default:
		return filament.WriteReceipt{}, fmt.Errorf("%s sink: write policy %q is not implemented", s.Name(), opts.Policy.Capability.Mode)
	}
	return s.write(ctx, batch)
}

func (s *Sink) write(ctx context.Context, batch *arrowbatch.Batch) (filament.WriteReceipt, error) {
	if s.db == nil {
		return filament.WriteReceipt{}, fmt.Errorf("%s sink: write before open", s.Name())
	}
	if batch.Rows() == nil {
		return filament.WriteReceipt{}, fmt.Errorf("%s sink: write requires a record batch", s.Name())
	}
	s.tablesMu.RLock()
	tbl := s.tables[batch.Resource]
	s.tablesMu.RUnlock()
	if tbl == nil {
		return filament.WriteReceipt{}, fmt.Errorf("%s sink: no schema ensured for resource %q", s.Name(), batch.Resource)
	}
	conn, err := s.take(ctx)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("%s sink: acquire connection: %w", s.Name(), err)
	}
	defer s.put(conn)
	if err := ctx.Err(); err != nil {
		return filament.WriteReceipt{}, err
	}
	appender := conn.appenders[batch.Resource]
	if appender == nil {
		appender, err = duckdb.NewAppender(conn.conn, s.database, s.schema, tbl.writeName)
		if err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("%s sink: create appender for %s: %w", s.Name(), batch.Resource, err)
		}
		conn.appenders[batch.Resource] = appender
	}
	rows := batch.Rows()
	writeCRC := batch.IntegrityCRC()
	if err := appendRows(appender, batch); err != nil {
		s.discardAppender(conn, batch.Resource)
		return filament.WriteReceipt{}, fmt.Errorf("%s sink: append %s seq %d: %w", s.Name(), batch.Resource, batch.Seq, err)
	}
	if err := appender.Flush(); err != nil {
		s.discardAppender(conn, batch.Resource)
		return filament.WriteReceipt{}, fmt.Errorf("%s sink: flush %s seq %d: %w", s.Name(), batch.Resource, batch.Seq, err)
	}
	s.written.Add(int64(batch.NumRows()))
	return filament.WriteReceipt{
		URI: s.tableURI(batch.Resource), Rows: batch.NumRows(), Bytes: arrowbatch.Bytes(rows), WriteCRC: writeCRC,
	}, nil
}

// appendRows hands every row of the batch to the appender, column by column
// through the converters chosen from the Arrow schema.
func appendRows(appender *duckdb.Appender, batch *arrowbatch.Batch) error {
	rows := batch.Rows()
	cols := rows.Columns()
	fns := make([]valueFn, len(cols))
	for i, f := range rows.Schema().Fields() {
		fns[i] = valueFor(f)
	}
	values := make([]driver.Value, len(cols))
	for i := range batch.NumRows() {
		for c, col := range cols {
			if col.IsNull(i) {
				values[c] = nil
				continue
			}
			values[c] = fns[c](col, i)
		}
		if err := appender.AppendRow(values...); err != nil {
			return fmt.Errorf("row %d: %w", i, err)
		}
	}
	return nil
}

func (s *Sink) discardAppender(conn *pooledConnection, resource string) {
	if appender := conn.appenders[resource]; appender != nil {
		_ = appender.Close()
		delete(conn.appenders, resource)
	}
}

func (s *Sink) tableURI(resource string) string {
	return fmt.Sprintf("md://%s/%s.%s", s.database, s.schema, resource)
}
