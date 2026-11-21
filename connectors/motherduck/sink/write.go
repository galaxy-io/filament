package motherduck

import (
	"context"
	"database/sql/driver"
	"fmt"
	"math/big"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	duckdb "github.com/marcboeker/go-duckdb/v2"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/internal/arrowtext"
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
	if batchHasEndOfDayTime(batch) {
		receipt, err := s.writeViaStatements(ctx, conn, batch, tbl)
		if err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("%s sink: write %s seq %d: %w", s.Name(), batch.Resource, batch.Seq, err)
		}
		s.written.Add(int64(batch.NumRows()))
		return receipt, nil
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
	if err := appendRows(appender, batch, tbl.columns); err != nil {
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

// writePositions resolves each source field to its destination position. With
// destination columns, each value goes to its column's position and columns
// the source lacks are written as NULL; without them, source order already
// matches.
func writePositions(fields []arrow.Field, destination []string) ([]int, int, error) {
	positions := make([]int, len(fields))
	for i := range fields {
		positions[i] = i
	}
	width := len(fields)
	if destination == nil {
		return positions, width, nil
	}
	index := make(map[string]int, len(destination))
	for i, name := range destination {
		index[strings.ToLower(name)] = i
	}
	for i, f := range fields {
		position, ok := index[strings.ToLower(f.Name)]
		if !ok {
			return nil, 0, fmt.Errorf("column %q is missing from the destination", f.Name)
		}
		positions[i] = position
	}
	return positions, len(destination), nil
}

// batchHasEndOfDayTime reports whether any TIME64 value in the batch is at or
// past 24:00:00. The appender binds TIME only from time.Time and reads the
// time of day off it, so such a value wraps to midnight; those batches go
// through prepared INSERTs with the time as text instead.
func batchHasEndOfDayTime(batch *arrowbatch.Batch) bool {
	rows := batch.Rows()
	fields := rows.Schema().Fields()
	for c, col := range rows.Columns() {
		if fields[c].Type.ID() != arrow.TIME64 {
			continue
		}
		values := col.(*array.Time64)
		for i := range batch.NumRows() {
			if col.IsNull(i) {
				continue
			}
			if int64(values.Value(i)) >= arrowtext.MicrosPerDay {
				return true
			}
		}
	}
	return false
}

// statementValue returns the value the prepared INSERT receives for one
// cell. TIME64 goes as text, decimals as exact text at the declared scale,
// and logical UUIDs as their original string. The statement binder rejects
// duckdb.Decimal outright and only accepts UUIDs through fmt.Stringer, which
// the duckdb.UUID value type does not implement, so those two cannot go
// through the appender converter. Everything else uses it unchanged.
func statementValue(f arrow.Field, col arrow.Array, i int, fn valueFn) driver.Value {
	switch f.Type.ID() {
	case arrow.TIME64:
		return string(arrowtext.AppendTimeOfDay(nil, int64(col.(*array.Time64).Value(i))))
	case arrow.DECIMAL128:
		scale := f.Type.(*arrow.Decimal128Type).Scale
		return decimalText(col.(*array.Decimal128).Value(i).BigInt(), int(scale))
	case arrow.STRING:
		if arrowbatch.LogicalOf(f) == rowmodel.LogicalUUID {
			return col.(*array.String).Value(i)
		}
	}
	return fn(col, i)
}

// decimalText renders an unscaled integer at the given scale, so 125 at
// scale 2 becomes "1.25" and 200 at scale 2 becomes "2.00".
func decimalText(value *big.Int, scale int) string {
	neg := value.Sign() < 0
	digits := new(big.Int).Abs(value).String()
	if scale == 0 {
		if neg {
			return "-" + digits
		}
		return digits
	}
	for len(digits) <= scale {
		digits = "0" + digits
	}
	text := digits[:len(digits)-scale] + "." + digits[len(digits)-scale:]
	if neg {
		return "-" + text
	}
	return text
}

// writeViaStatements writes one batch through a prepared INSERT, passing TIME
// values as text. DuckDB accepts "24:00:00" as a TIME value, which the
// appender path cannot carry: it binds TIME only from time.Time and reads the
// time of day off it, so 24:00:00 wraps to midnight. The batch writes in one
// transaction, so a failure leaves no partial rows behind. Every destination
// column is listed, with explicit NULLs for columns the source lacks, so the
// batch stores the same values as the appender path instead of letting a
// column default apply.
func (s *Sink) writeViaStatements(ctx context.Context, conn *pooledConnection, batch *arrowbatch.Batch, tbl *table) (filament.WriteReceipt, error) {
	rows := batch.Rows()
	cols := rows.Columns()
	fields := rows.Schema().Fields()
	fns := make([]valueFn, len(cols))
	for i, f := range fields {
		fns[i] = valueFor(f)
	}
	positions, width, err := writePositions(fields, tbl.columns)
	if err != nil {
		return filament.WriteReceipt{}, err
	}
	names := make([]string, width)
	placeholders := make([]string, width)
	if tbl.columns != nil {
		for i, name := range tbl.columns {
			names[i] = quoteIdent(name)
			placeholders[i] = "?"
		}
	} else {
		for i, f := range fields {
			names[i] = quoteIdent(f.Name)
			placeholders[i] = "?"
		}
	}
	query := "INSERT INTO " + tbl.writeTo + " (" + strings.Join(names, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
	preparer, ok := conn.conn.(driver.ConnPrepareContext)
	if !ok {
		return filament.WriteReceipt{}, fmt.Errorf("connection does not support prepared statements")
	}
	stmt, err := preparer.PrepareContext(ctx, query)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("prepare insert for %s: %w", batch.Resource, err)
	}
	defer func() { _ = stmt.Close() }()
	if err := conn.exec(ctx, "BEGIN TRANSACTION"); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("begin transaction for %s: %w", batch.Resource, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = conn.exec(context.WithoutCancel(ctx), "ROLLBACK")
		}
	}()
	values := make([]driver.Value, width)
	for i := range batch.NumRows() {
		for j := range values {
			values[j] = nil
		}
		for c, col := range cols {
			if col.IsNull(i) {
				continue
			}
			values[positions[c]] = statementValue(fields[c], col, i, fns[c])
		}
		if err := execStmt(ctx, stmt, values); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("row %d: %w", i, err)
		}
	}
	if err := conn.exec(ctx, "COMMIT"); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("commit %s seq %d: %w", batch.Resource, batch.Seq, err)
	}
	committed = true
	return filament.WriteReceipt{
		URI: s.tableURI(batch.Resource), Rows: batch.NumRows(), Bytes: arrowbatch.Bytes(rows), WriteCRC: batch.IntegrityCRC(),
	}, nil
}

func execStmt(ctx context.Context, stmt driver.Stmt, values []driver.Value) error {
	execer, ok := stmt.(driver.StmtExecContext)
	if !ok {
		return fmt.Errorf("statement does not support ExecContext")
	}
	named := make([]driver.NamedValue, len(values))
	for i, value := range values {
		named[i] = driver.NamedValue{Ordinal: i + 1, Value: value}
	}
	_, err := execer.ExecContext(ctx, named)
	return err
}

// appendRows hands every row of the batch to the appender, column by column
// through the converters chosen from the Arrow schema. With destination
// columns, each value goes to its column's position and columns the source
// lacks are written as NULL.
func appendRows(appender *duckdb.Appender, batch *arrowbatch.Batch, destination []string) error {
	rows := batch.Rows()
	cols := rows.Columns()
	fields := rows.Schema().Fields()
	fns := make([]valueFn, len(cols))
	for i, f := range fields {
		fns[i] = valueFor(f)
	}
	positions, width, err := writePositions(fields, destination)
	if err != nil {
		return err
	}
	values := make([]driver.Value, width)
	for i := range batch.NumRows() {
		for c, col := range cols {
			if col.IsNull(i) {
				values[positions[c]] = nil
				continue
			}
			values[positions[c]] = fns[c](col, i)
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
