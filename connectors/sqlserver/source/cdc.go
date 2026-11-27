package sqlserver

// Change data capture. A run is a bounded catch-up cycle, as for the
// PostgreSQL and MySQL sources: it takes a watermark, replays every committed
// change after each table's checkpoint up to it, drains a stream mark for
// every table at the position it reached, and returns. The next run resumes
// from those marks.
//
// # One ordered read across tables
//
// Every table's changes are read by one query: a UNION ALL with one branch
// per change table, each a range seek on the change table's clustered index
// (__$start_lsn, __$command_id, __$seqval, __$operation), ordered by those
// columns. The server merges the already-ordered branches (Merge Join
// (Concatenation), no sort), so changes arrive in commit order across tables,
// a transaction's rows in the order it made them, over one connection, and
// stream through RawRows like a snapshot read. Each branch fills its own
// table's columns and leaves the others' typed NULLs. A selection wider than
// one SELECT allows is split into groups read one after another.
//
// # Positions
//
// __$start_lsn is the commit LSN of a change's transaction. A change row is
// stamped with the LSN just before it, so a checkpoint flushed mid-transaction
// re-reads the whole transaction on resume. A stream mark carries the
// position the table is complete through.
//
// # Initial snapshot
//
// A table without a checkpoint is read in full first, by the keyset reader a
// full refresh uses, shards in parallel. Its floor, the position its changes
// are read after, is taken before the snapshot starts, so a change that races
// the snapshot is replayed rather than missed; replay is idempotent under the
// merge policy. Change tables trail the log by the capture
// process's latency, so after the snapshot the reader waits for capture to
// pass the log position the snapshot ended at before taking the watermark.
// Without that wait, replay could stop short of rows the snapshot already
// holds and briefly write older versions over them.

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mssql "github.com/microsoft/go-mssqldb"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/rowmodel"
)

var _ filament.ChangeSource = (*Source)(nil)

const (
	// maxSelectColumns stays under SQL Server's 4096 columns per SELECT.
	maxSelectColumns = 4000
	// maxBranches keeps a group's two range parameters per branch under the
	// 2100-parameter limit.
	maxBranches = 1000
	// changeHeader is the leading columns of a change read: start LSN,
	// command id, sequence value, operation and the branch's table ordinal.
	changeHeader = 5

	// defaultSnapshotParallelism is how many shards an initial snapshot reads
	// at once when max_conns is not set.
	defaultSnapshotParallelism = 4

	captureWait = 2 * time.Minute
	capturePoll = 500 * time.Millisecond
)

// CDC change-table operations.
const (
	opDelete       = 1
	opInsert       = 2
	opUpdateBefore = 3
	opUpdateAfter  = 4
)

// cdcTable is one tracked resource.
type cdcTable struct {
	t         *table
	instances []captureInstance
	floor     lsn // changes at or before floor are already delivered
	through   lsn // this cycle has read every change at or before through
	plan      readPlan
	writer    arrowbatch.RowWriter
}

type cdcRun struct {
	sink    arrowbatch.Inlet
	tables  []*cdcTable
	byName  map[string]*cdcTable
	seq     uint64
	emitted int
	limit   int
	limited bool
}

func (r *cdcRun) writer(t *cdcTable) (arrowbatch.RowWriter, error) {
	if t.writer == nil {
		w, err := r.sink.Builder(t.t.name, 0, t.t.schema)
		if err != nil {
			return nil, err
		}
		t.writer = w
	}
	return t.writer, nil
}

// ExtractChanges replays committed changes to the selected tables from their
// checkpoints to the current capture position, snapshotting first any table
// that has no checkpoint yet.
func (s *Source) ExtractChanges(ctx context.Context, sink arrowbatch.Inlet, opts filament.ChangeExtractOpts) error {
	if s.db == nil {
		return fmt.Errorf("sqlserver source: extract changes before configure")
	}
	if len(opts.Resources) == 0 {
		return nil
	}
	if err := s.requireCDC(ctx); err != nil {
		return err
	}
	run, err := s.trackTables(ctx, sink, opts)
	if err != nil {
		return err
	}
	bootstrap, err := run.resume(opts.Checkpoints)
	if err != nil {
		return err
	}
	watermark, err := s.maxCapturedLSN(ctx)
	if err != nil {
		return err
	}
	if len(bootstrap) > 0 {
		// A floor below the newest instance's start would ask for changes it
		// never captured; changes before it began are in the snapshot.
		for _, ct := range bootstrap {
			ct.floor = maxLSN(watermark, ct.instances[len(ct.instances)-1].start)
		}
		if s.snapshotMode != filament.SnapshotNone {
			if err := s.snapshotTables(ctx, run, bootstrap); err != nil {
				return err
			}
			if watermark, err = s.awaitCapture(ctx); err != nil {
				return err
			}
			// The snapshot holds rows as of its end, and changes since the
			// floor may be older. Replay must reach the watermark to bring
			// them back up to date, so a cycle that snapshots replays in
			// full; the snapshot itself is not limited either.
			run.limit = 0
		}
	}
	if err := s.checkTimeline(ctx, run.tables); err != nil {
		return err
	}
	if err := s.replay(ctx, run, watermark); err != nil {
		return err
	}
	return run.pushStreamMarks()
}

// trackTables reads each selected table's catalog and capture instances.
func (s *Source) trackTables(ctx context.Context, sink arrowbatch.Inlet, opts filament.ChangeExtractOpts) (*cdcRun, error) {
	run := &cdcRun{sink: sink, byName: map[string]*cdcTable{}, limit: opts.Limit}
	for _, name := range opts.Resources {
		t, err := s.tableMeta(ctx, name)
		if err != nil {
			return nil, err
		}
		if len(t.pks) == 0 {
			return nil, fmt.Errorf("sqlserver cdc: table %s has no primary key", t.qualified)
		}
		instances, err := s.captureInstances(ctx, t)
		if err != nil {
			return nil, err
		}
		ct := &cdcTable{t: t, instances: instances}
		run.tables = append(run.tables, ct)
		run.byName[name] = ct
	}
	return run, nil
}

// resume sets each checkpointed table's floor and the run's sequence, and
// returns the tables that have no checkpoint yet.
func (r *cdcRun) resume(cps map[string]filament.Checkpoint) ([]*cdcTable, error) {
	var bootstrap []*cdcTable
	for _, ct := range r.tables {
		raw, seq, ok := checkpoint.ParseStream(cps[ct.t.name])
		if !ok {
			if cps[ct.t.name] != nil {
				return nil, fmt.Errorf("sqlserver cdc: checkpoint for %q is not a stream cursor", ct.t.name)
			}
			bootstrap = append(bootstrap, ct)
			continue
		}
		floor, err := parseLSN(raw)
		if err != nil {
			return nil, fmt.Errorf("sqlserver cdc: checkpoint for %q: %w", ct.t.name, err)
		}
		ct.floor = floor
		r.seq = max(r.seq, seq)
	}
	return bootstrap, nil
}

// replay reads every table's changes after its floor through watermark,
// recording in each table how far it got. All tables advance together, one
// interval at a time, up to the next point where a table moves from an older
// capture instance to its newer one. A table's changes after that point are
// in the newer instance, so reading other tables past it first would emit
// their later changes ahead of its earlier ones.
func (s *Source) replay(ctx context.Context, run *cdcRun, watermark lsn) error {
	var err error
	for _, ct := range run.tables {
		if ct.plan, err = planRead(ct.t, ct.instances, ct.floor); err != nil {
			return err
		}
	}
	for {
		next := watermark
		for _, ct := range run.tables {
			if ct.plan.rotating && ct.plan.stop.compare(next) < 0 {
				next = ct.plan.stop
			}
		}
		floors := make([]lsn, len(run.tables))
		for i, ct := range run.tables {
			floors[i] = ct.floor
		}
		reached, err := s.readChanges(ctx, run, run.tables, next)
		if err != nil {
			return err
		}
		if err := s.checkRetained(ctx, run.tables, floors); err != nil {
			return err
		}
		for _, ct := range run.tables {
			ct.floor = maxLSN(ct.floor, ct.through)
		}
		if run.limited || reached.compare(watermark) >= 0 {
			return nil
		}
		for _, ct := range run.tables {
			if ct.plan.rotating && ct.plan.stop.compare(reached) <= 0 {
				if ct.plan, err = planRead(ct.t, ct.instances, ct.plan.stop); err != nil {
					return err
				}
			}
		}
	}
}

// checkRetained fails when cleanup raised the low watermark of an instance a
// read used above the first change the read asked for. planRead checked the
// watermark before, but cleanup can delete changes after that, and the read
// then returns only what is left without any sign of the gap.
func (s *Source) checkRetained(ctx context.Context, tables []*cdcTable, floors []lsn) error {
	rows, err := s.db.QueryContext(ctx, `SELECT capture_instance, start_lsn FROM cdc.change_tables`)
	if err != nil {
		return fmt.Errorf("sqlserver cdc: capture instance watermarks: %w", err)
	}
	defer func() { _ = rows.Close() }()
	starts := map[string]lsn{}
	for rows.Next() {
		var name string
		var raw []byte
		if err := rows.Scan(&name, &raw); err != nil {
			return fmt.Errorf("sqlserver cdc: capture instance watermarks: %w", err)
		}
		if starts[name], err = lsnFromBytes(raw); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("sqlserver cdc: capture instance watermarks: %w", err)
	}
	for i, ct := range tables {
		inst := ct.plan.inst
		start, ok := starts[inst.name]
		switch {
		case !ok:
			return fmt.Errorf("sqlserver cdc: capture instance %q of %s was dropped while its changes were read; run again to read %s from the instance that replaces it",
				inst.name, ct.t.qualified, ct.t.qualified)
		case start.compare(floors[i].next()) > 0:
			return fmt.Errorf("sqlserver cdc: changes to %s after %s were removed from capture instance %q while they were read (its oldest retained change is now %s); "+
				"cleanup removed them, so the table must be snapshotted again", ct.t.qualified, floors[i], inst.name, start)
		}
	}
	return nil
}

// readChanges replays every table's changes in (floor, min(end, stop)], sets
// each read table's through, and returns where later reads stop: end, or the
// last whole transaction when the row limit cut the read short.
func (s *Source) readChanges(ctx context.Context, run *cdcRun, tables []*cdcTable, end lsn) (lsn, error) {
	var pending []*cdcTable
	for _, ct := range tables {
		if ct.floor.compare(ct.upper(end)) < 0 {
			pending = append(pending, ct)
		}
	}
	if len(pending) == 0 {
		return end, nil
	}
	stream := &changeStream{run: run, stopAtLimit: !run.limited}
	var readers []*changeReader
	var queries []string
	var args [][]any
	for _, group := range changeGroups(pending) {
		query, a := changeQuery(group, end)
		readers = append(readers, &changeReader{stream: stream, tables: group})
		queries, args = append(queries, query), append(args, a)
	}
	var err error
	if len(readers) == 1 {
		err = s.streamChanges(ctx, readers[0], queries[0], args[0])
	} else {
		err = s.readMerged(ctx, readers, queries, args)
	}
	switch {
	case errors.Is(err, errLimit):
		end = stream.done
	case err != nil:
		return lsn{}, fmt.Errorf("sqlserver cdc: read changes: %w", err)
	case stream.stopAtLimit && run.limited:
		end = stream.cur // the limit fell in the last transaction, read whole
	}
	for _, ct := range pending {
		ct.through = ct.upper(end)
	}
	return end, nil
}

// streamChanges runs one change query, handing its rows to h as they arrive.
func (s *Source) streamChanges(ctx context.Context, h mssql.RawRowHandler, query string, args []any) error {
	rows, err := s.db.QueryContext(ctx, query, append(args, mssql.RawRows{Handler: h})...)
	if err == nil {
		for rows.Next() { //nolint:revive // rows are delivered to the handler
		}
		err = errors.Join(rows.Err(), rows.Close())
	}
	return err
}

// readMerged reads a selection too wide for one query: one query per group,
// all at once on their own connections, merged in the order one query would
// return them. Reading the groups one after another would emit a later
// group's earlier commits after an earlier group's later ones.
func (s *Source) readMerged(ctx context.Context, readers []*changeReader, queries []string, args [][]any) error {
	if s.maxConns > 0 && s.maxConns < len(readers) {
		return fmt.Errorf("the selection needs %d queries read at once to keep commit order, but max_conns is %d", len(readers), s.maxConns)
	}
	produce := func(ctx context.Context, i int, out chan<- *bufferedChange) error {
		return s.streamChanges(ctx, &changeTap{ctx: ctx, reader: readers[i], out: out}, queries[i], args[i])
	}
	apply := func(i int, c *bufferedChange) error { return readers[i].stream.apply(readers[i], c) }
	return mergeChanges(ctx, len(readers), produce, apply)
}

// mergeChanges runs n producers at once and applies their rows in change
// order until every producer ends or apply fails. It holds one row per
// producer, each one's next, so memory stays at a row per producer however
// far one could run ahead.
//
// A producer that fails before the merge stops has ended early, and the rows
// it did not deliver are missing from the merge. That is a failure even when
// apply stopped at the row limit, which would otherwise advance every table
// past changes never read. A producer that fails after the merge stopped was
// cancelled by the stop.
func mergeChanges(ctx context.Context, n int, produce func(context.Context, int, chan<- *bufferedChange) error, apply func(int, *bufferedChange) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	feeds := make([]chan *bufferedChange, n)
	errs := make([]error, n)
	var stopped atomic.Bool
	var wg sync.WaitGroup
	for i := range n {
		feeds[i] = make(chan *bufferedChange)
		wg.Go(func() {
			defer close(feeds[i])
			if err := produce(ctx, i, feeds[i]); err != nil && !stopped.Load() {
				errs[i] = err
			}
		})
	}
	heads := make([]*bufferedChange, n)
	for i := range feeds {
		heads[i] = <-feeds[i]
	}
	var stopErr error
	for {
		next := -1
		for i, h := range heads {
			if h != nil && (next < 0 || h.before(heads[next])) {
				next = i
			}
		}
		if next < 0 {
			break
		}
		if stopErr = apply(next, heads[next]); stopErr != nil {
			break
		}
		heads[next] = <-feeds[next]
	}
	stopped.Store(true)
	cancel()
	for _, feed := range feeds {
		for range feed { //nolint:revive // drained so the producers can return
		}
	}
	wg.Wait()
	var failed []error
	for _, err := range errs {
		if err != nil {
			failed = append(failed, err)
		}
	}
	if len(failed) == 0 {
		return stopErr
	}
	if stopErr != nil && !errors.Is(stopErr, errLimit) {
		failed = append([]error{stopErr}, failed...)
	}
	return errors.Join(failed...)
}

// upper is where a cycle ending at end stops reading this table.
func (t *cdcTable) upper(end lsn) lsn {
	if t.plan.stop.isZero() {
		return end
	}
	return minLSN(end, t.plan.stop)
}

// changeGroups splits tables into groups that each fit one SELECT.
func changeGroups(tables []*cdcTable) [][]*cdcTable {
	var groups [][]*cdcTable
	group := make([]*cdcTable, 0, len(tables))
	width := changeHeader
	for _, ct := range tables {
		n := len(ct.t.schema.Fields)
		if len(group) > 0 && (width+n > maxSelectColumns || len(group) >= maxBranches) {
			groups = append(groups, group)
			group, width = make([]*cdcTable, 0, len(tables)), changeHeader
		}
		group = append(group, ct)
		width += n
	}
	if len(group) > 0 {
		groups = append(groups, group)
	}
	return groups
}

// changeQuery builds the ordered read of tables' changes in (floor, upper].
func changeQuery(tables []*cdcTable, end lsn) (string, []any) {
	var b strings.Builder
	args := make([]any, 0, 2*len(tables))
	b.WriteString("SELECT * FROM (")
	for j, ct := range tables {
		if j > 0 {
			b.WriteString("\nUNION ALL\n")
		}
		inst := ct.plan.inst
		commandID := "CAST(0 AS int)"
		if inst.commandID {
			commandID = "[__$command_id]"
		}
		fmt.Fprintf(&b, "SELECT [__$start_lsn] AS [s], %s AS [c], [__$seqval] AS [q], [__$operation] AS [o], %d AS [t]", commandID, j)
		for k, other := range tables {
			for i, f := range other.t.schema.Fields {
				b.WriteString(", ")
				if k == j {
					b.WriteString(capturedExpr(ct, f.Name))
				} else {
					b.WriteString(nullExpr(other, f.Name))
				}
				if j == 0 {
					fmt.Fprintf(&b, " AS [f%d_%d]", k, i)
				}
			}
		}
		args = append(args, ct.floor[:], ct.upper(end).bytes())
		fmt.Fprintf(&b, " FROM %s WHERE [__$start_lsn] > @p%d AND [__$start_lsn] <= @p%d", inst.ct, len(args)-1, len(args))
	}
	b.WriteString(") x ORDER BY [s], [c], [q], [o]")
	return b.String(), args
}

func (l lsn) bytes() []byte { return l[:] }

// capturedExpr selects one field from a table's change table: the captured
// column, converted as a snapshot converts it, or a typed NULL for a column
// only the instance replacing this one captures.
func capturedExpr(ct *cdcTable, name string) string {
	c, ok := ct.plan.inst.cols[name]
	if !ok {
		return nullExpr(ct, name)
	}
	if st := typeFor(c); st.selectExpr != nil {
		return st.selectExpr(quoteIdent(name))
	}
	return quoteIdent(name)
}

// nullExpr is a NULL of the type the field has in ct's read, so every branch
// of the union agrees on each column's type.
func nullExpr(ct *cdcTable, name string) string {
	c, ok := ct.plan.inst.cols[name]
	if !ok {
		for _, live := range ct.t.cols {
			if live.name == name {
				c = live
			}
		}
	}
	decl := c.decl()
	if isRowversion(decl) {
		decl = "binary(8)" // how change tables store rowversion
	}
	expr := "CAST(NULL AS " + decl + ")"
	if st := typeFor(c); st.selectExpr != nil {
		expr = st.selectExpr(expr)
	}
	return expr
}

// pushStreamMarks drains every table's writer at the position it is complete
// through, so the next cycle resumes there even for a table that saw no
// change. A mark never falls below the table's floor: a cut-short cycle can
// stop before a newly snapshotted table's floor, and marking it lower would
// replay changes its snapshot holds.
func (r *cdcRun) pushStreamMarks() error {
	r.seq++ // the final position outranks every row even when folds arrive out of order
	for _, ct := range r.tables {
		w, err := r.writer(ct)
		if err != nil {
			return err
		}
		mark := maxLSN(ct.floor, ct.through)
		if err := w.Drain(rowmodel.Meta{LSN: mark.String(), Seq: r.seq}); err != nil {
			return err
		}
	}
	return nil
}

// changeStream is the state a cycle's change read carries from row to row,
// across every query of a merged read: the transaction being read, where the
// row limit stops it, and an update's pre-image.
type changeStream struct {
	run *cdcRun
	// stopAtLimit ends the read at the first transaction boundary after the
	// run's row limit is reached.
	stopAtLimit bool

	cur  lsn // commit LSN of the transaction being read
	done lsn // last transaction read in full

	// before is the pre-image of an update (operation 3), kept until its
	// post-image arrives to tell whether the update changed the key.
	before       [][]byte
	beforeNull   []bool
	beforeSeq    []byte
	beforeReader *changeReader
	beforeTable  int
	hasBefore    bool
}

// changeValues is one change row's columns: a row streaming from the server,
// or one copied out of it for a merged read.
type changeValues interface {
	Value(i int) ([]byte, bool)
}

// changeReader is the RawRowHandler of one change query. It routes each row
// to its table's writer by the branch ordinal and decodes the table's columns
// straight from the TDS bytes.
type changeReader struct {
	stream *changeStream
	tables []*cdcTable

	offsets []int // first result column of each table's fields
	decs    [][]decodeFunc
	keys    [][]int // field index of each primary-key column
	scratch rowReader
}

func (r *changeReader) Columns(cols []mssql.RawColumn) error {
	want := changeHeader
	for _, ct := range r.tables {
		want += len(ct.t.schema.Fields)
	}
	if len(cols) != want {
		return fmt.Errorf("change read returned %d columns, want %d", len(cols), want)
	}
	r.offsets = make([]int, len(r.tables))
	r.decs = make([][]decodeFunc, len(r.tables))
	r.keys = make([][]int, len(r.tables))
	at := changeHeader
	for k, ct := range r.tables {
		fields := ct.t.schema.Fields
		r.offsets[k] = at
		r.decs[k] = make([]decodeFunc, len(fields))
		for i := range fields {
			dec, err := decoderFor(&cols[at+i], fields[i])
			if err != nil {
				return fmt.Errorf("%s: %w", ct.t.qualified, err)
			}
			r.decs[k][i] = dec
		}
		for _, pk := range ct.t.schema.PrimaryKey {
			for i, f := range fields {
				if f.Name == pk {
					r.keys[k] = append(r.keys[k], i)
				}
			}
		}
		at += len(fields)
	}
	return nil
}

func (r *changeReader) Row(row *mssql.RawRow) error { return r.stream.apply(r, row) }

// apply folds one change row of r's query into the stream.
func (st *changeStream) apply(r *changeReader, row changeValues) error {
	raw, _ := row.Value(0)
	commit, err := lsnFromBytes(raw)
	if err != nil {
		return err
	}
	if commit != st.cur {
		if !st.cur.isZero() {
			st.done = st.cur
		}
		if st.stopAtLimit && st.run.limited {
			return errLimit // stop on a transaction boundary
		}
		st.cur = commit
	}
	op, err := intAt(row, 3)
	if err != nil {
		return err
	}
	k, err := intAt(row, 4)
	if err != nil || k < 0 || int(k) >= len(r.tables) {
		return fmt.Errorf("change row has table ordinal %d: %w", k, err)
	}
	table := int(k)
	seqval, _ := row.Value(2)
	live := func(i int) ([]byte, bool) { return row.Value(r.offsets[table] + i) }

	switch op {
	case opUpdateBefore:
		st.keepBefore(r, table, seqval, live)
		return nil
	case opUpdateAfter:
		if st.hasBefore && st.beforeReader == r && st.beforeTable == table && bytes.Equal(st.beforeSeq, seqval) && st.keyChanged(r, table, live) {
			// A key change is a delete of the old row and an insert of the new.
			st.hasBefore = false
			if err := r.emit(table, st.stashed, rowmodel.OpDelete); err != nil {
				return err
			}
			return r.emit(table, live, rowmodel.OpInsert)
		}
		st.hasBefore = false
		return r.emit(table, live, rowmodel.OpUpdate)
	case opDelete:
		return r.emit(table, live, rowmodel.OpDelete)
	case opInsert:
		return r.emit(table, live, rowmodel.OpInsert)
	}
	return fmt.Errorf("change row has operation %d", op)
}

func (st *changeStream) keepBefore(r *changeReader, table int, seqval []byte, value func(int) ([]byte, bool)) {
	n := len(r.tables[table].t.schema.Fields)
	if cap(st.before) < n {
		st.before = make([][]byte, n)
		st.beforeNull = make([]bool, n)
	}
	st.before, st.beforeNull = st.before[:n], st.beforeNull[:n]
	for i := range n {
		v, ok := value(i)
		st.beforeNull[i] = !ok
		st.before[i] = append(st.before[i][:0], v...)
	}
	st.beforeSeq = append(st.beforeSeq[:0], seqval...)
	st.beforeReader, st.beforeTable = r, table
	st.hasBefore = true
}

func (st *changeStream) stashed(i int) ([]byte, bool) { return st.before[i], !st.beforeNull[i] }

func (st *changeStream) keyChanged(r *changeReader, table int, after func(int) ([]byte, bool)) bool {
	for _, i := range r.keys[table] {
		v, ok := after(i)
		if ok == st.beforeNull[i] || !bytes.Equal(v, st.before[i]) {
			return true
		}
	}
	return false
}

// emit appends one change row, stamped with the position just before its
// transaction's commit.
func (r *changeReader) emit(table int, value func(int) ([]byte, bool), op rowmodel.Operation) error {
	ct := r.tables[table]
	if missing := ct.plan.uncaptured; len(missing) > 0 {
		// NULL would overwrite the row's real value in the destination, and
		// the row model cannot say "unknown".
		return fmt.Errorf("a change to %s at %s is in capture instance %q, which does not capture %s, so the change's value for it is unknown; "+
			"snapshot the table again to read it from the instance that does",
			ct.t.qualified, r.stream.cur, ct.plan.inst.name, quoteNames(missing))
	}
	run := r.stream.run
	w, err := run.writer(ct)
	if err != nil {
		return err
	}
	for i, dec := range r.decs[table] {
		v, ok := value(i)
		if !ok {
			w.Null()
			continue
		}
		if err := dec(&r.scratch, w, v); err != nil {
			return fmt.Errorf("%s column %q: %w", ct.t.qualified, ct.t.schema.Fields[i].Name, err)
		}
	}
	run.seq++
	if err := w.EndRow(rowmodel.Meta{Op: op, LSN: r.stream.cur.prev().String(), Seq: run.seq}); err != nil {
		return err
	}
	run.emitted++
	if run.limit > 0 && run.emitted >= run.limit {
		run.limited = true
	}
	return nil
}

// changeTap is the RawRowHandler of one query of a merged read. It copies
// each row out of the TDS buffer, which the driver reuses, and hands it to
// the merge.
type changeTap struct {
	ctx    context.Context
	reader *changeReader
	out    chan<- *bufferedChange
	width  int
}

func (t *changeTap) Columns(cols []mssql.RawColumn) error {
	t.width = len(cols)
	return t.reader.Columns(cols)
}

func (t *changeTap) Row(row *mssql.RawRow) error {
	c := &bufferedChange{values: make([][]byte, t.width), null: make([]bool, t.width)}
	for i := range t.width {
		v, ok := row.Value(i)
		c.values[i], c.null[i] = bytes.Clone(v), !ok
	}
	select {
	case t.out <- c:
		return nil
	case <-t.ctx.Done():
		return t.ctx.Err()
	}
}

// bufferedChange is a change row copied out of its query.
type bufferedChange struct {
	values [][]byte
	null   []bool
}

func (c *bufferedChange) Value(i int) ([]byte, bool) { return c.values[i], !c.null[i] }

// before orders rows as a change query does: by commit LSN, command id,
// sequence value and operation.
func (c *bufferedChange) before(o *bufferedChange) bool {
	if d := bytes.Compare(c.values[0], o.values[0]); d != 0 {
		return d < 0
	}
	if a, b := headerInt(c, 1), headerInt(o, 1); a != b {
		return a < b
	}
	if d := bytes.Compare(c.values[2], o.values[2]); d != 0 {
		return d < 0
	}
	return headerInt(c, 3) < headerInt(o, 3)
}

func headerInt(c *bufferedChange, i int) int64 {
	n, _ := readInt(c.values[i])
	return n
}

func quoteNames(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = fmt.Sprintf("%q", n)
	}
	return strings.Join(quoted, ", ")
}

func intAt(row changeValues, i int) (int64, error) {
	v, ok := row.Value(i)
	if !ok {
		return 0, fmt.Errorf("change column %d is NULL", i)
	}
	return readInt(v)
}

// maxCapturedLSN is the newest commit the capture process has written to the
// change tables, zero before it has captured anything.
func (s *Source) maxCapturedLSN(ctx context.Context) (lsn, error) {
	var raw []byte
	if err := s.db.QueryRowContext(ctx, `SELECT sys.fn_cdc_get_max_lsn()`).Scan(&raw); err != nil {
		return lsn{}, fmt.Errorf("sqlserver cdc: max captured lsn: %w", err)
	}
	return lsnFromBytes(raw)
}

// logEnd reads the end of the database log and the server's clock. It
// reports false when the end is unknown: the login lacks the permission to
// read it, or the database is a readable secondary, which does not report
// it. An unknown end proves nothing, so callers fall back as they do without
// the permission.
func (s *Source) logEnd(ctx context.Context) (lsn, string, bool, error) {
	var raw sql.NullString
	var now string
	err := s.db.QueryRowContext(ctx, `SELECT log_end_lsn, CONVERT(nvarchar(27), SYSDATETIME(), 126) FROM sys.dm_db_log_stats(DB_ID())`).Scan(&raw, &now)
	if isPermissionError(err) {
		return lsn{}, "", false, nil
	}
	if err != nil {
		return lsn{}, "", false, fmt.Errorf("sqlserver cdc: log end: %w", err)
	}
	if !raw.Valid {
		return lsn{}, "", false, nil
	}
	end, err := parseLSN(raw.String)
	if err != nil {
		return lsn{}, "", false, fmt.Errorf("sqlserver cdc: log end: %w", err)
	}
	return end, now, true, nil
}

// awaitCapture returns a watermark that covers every transaction committed
// before it was called: the max captured LSN once it passes the log end, or
// the log end itself once a capture scan that started after the log end was
// read has found nothing to capture (an idle database's max LSN does not
// move, since only transactions that touch captured tables reach the change
// tables, and no progress row is written for an empty scan). When the log
// end is unknown, the current max captured LSN is returned; a change capture
// has not reached yet is then replayed by the next cycle.
func (s *Source) awaitCapture(ctx context.Context) (lsn, error) {
	target, since, ok, err := s.logEnd(ctx)
	if err != nil || !ok {
		if err == nil {
			return s.maxCapturedLSN(ctx)
		}
		return lsn{}, err
	}
	baseline, err := s.captureProgress(ctx, since)
	if isPermissionError(err) {
		return s.maxCapturedLSN(ctx)
	}
	if err != nil {
		return lsn{}, err
	}
	deadline := time.Now().Add(captureWait)
	for {
		now, err := s.captureProgress(ctx, since)
		if err != nil {
			return lsn{}, err
		}
		switch {
		case now.captured.compare(target) >= 0:
			return now.captured, nil
		case now.idleSince(baseline):
			return maxLSN(now.captured, target), nil
		case time.Now().After(deadline):
			return lsn{}, fmt.Errorf("sqlserver cdc: capture has not reached log position %s after %s (captured through %s); check that the capture job is running", target, captureWait, now.captured)
		}
		select {
		case <-ctx.Done():
			return lsn{}, ctx.Err()
		case <-time.After(capturePoll):
		}
	}
}

// captureProgress is the capture process's position and its latest scan.
type captureProgress struct {
	captured lsn
	// session is the latest real scan session (session 0 aggregates every
	// session), zero before capture has scanned; empty counts the consecutive
	// scans that session found nothing in, and fresh reports that it started
	// after the log end was read. The DMV's start time is a datetime, which
	// rounds to the nearest 3.33 ms and can land after the log-end sample
	// when the scan began before it, so fresh needs a start more than 4 ms
	// after the sample. A scan closer than that counts by its empty scans.
	session int64
	empty   int64
	fresh   bool
}

// idleSince reports that a scan which started after baseline was read found
// nothing to capture. SQL Server folds consecutive empty scans into one
// session row whose start time stays at the first, so a fresh session counts
// at once, and the same session counts after two more empty scans: scans run
// one at a time, so the second started after baseline.
func (p captureProgress) idleSince(baseline captureProgress) bool {
	if p.empty == 0 {
		return false
	}
	return p.fresh || (p.session == baseline.session && p.empty >= baseline.empty+2)
}

func (s *Source) captureProgress(ctx context.Context, since string) (captureProgress, error) {
	const q = `SELECT sys.fn_cdc_get_max_lsn(), l.session_id, l.empty_scan_count,
	CASE WHEN l.start_time > DATEADD(millisecond, 4, CAST(@p1 AS datetime2)) THEN 1 ELSE 0 END
FROM (SELECT 1 AS one) d
OUTER APPLY (
	SELECT TOP (1) session_id, empty_scan_count, start_time
	FROM sys.dm_cdc_log_scan_sessions WHERE session_id > 0 ORDER BY session_id DESC
) l`
	var raw []byte
	var session, empty sql.NullInt64
	var fresh sql.NullBool
	if err := s.db.QueryRowContext(ctx, q, since).Scan(&raw, &session, &empty, &fresh); err != nil {
		if isPermissionError(err) {
			return captureProgress{}, err
		}
		return captureProgress{}, fmt.Errorf("sqlserver cdc: capture progress: %w", err)
	}
	captured, err := lsnFromBytes(raw)
	if err != nil {
		return captureProgress{}, err
	}
	return captureProgress{captured: captured, session: session.Int64, empty: empty.Int64, fresh: fresh.Bool}, nil
}

// checkTimeline rejects a checkpoint ahead of the end of the log, which
// means the database was restored or replaced under the stream. Skipped
// when the log end is unknown.
func (s *Source) checkTimeline(ctx context.Context, tables []*cdcTable) error {
	end, _, ok, err := s.logEnd(ctx)
	if err != nil || !ok {
		return err
	}
	for _, ct := range tables {
		if ct.floor.compare(end) > 0 {
			return fmt.Errorf("sqlserver cdc: checkpoint %s for %q is ahead of the database log end %s; was the database restored?", ct.floor, ct.t.name, end)
		}
	}
	return nil
}

func isPermissionError(err error) bool {
	var e mssql.Error
	if !errors.As(err, &e) {
		return false
	}
	switch e.Number {
	case 297, 300, 262, 229: // no permission to see the DMV or run the query
		return true
	}
	return false
}

// snapshotTables reads tables in full with the keyset reader a full refresh
// uses: every table's shards, up to snapshotParallelism at a time, each on its
// own connection, in its own read transaction and into its own part.
//
// Snapshot rows must reach the sink before the change rows that follow them,
// or an old snapshot row could overwrite a newer change. A CDC run writes
// through a single pipeline writer, which applies batches in the order they
// are queued, so it is enough to flush every shard's builder before replay
// starts. Snapshot rows are inserts with no position: the stream mark after
// them is what checkpoints the table. Limit does not apply, since a partial
// snapshot followed by a mark would skip rows for good.
func (s *Source) snapshotTables(ctx context.Context, run *cdcRun, tables []*cdcTable) error {
	inlet := &snapshotInlet{run: run}
	var jobs []job
	for _, ct := range tables {
		tjobs, err := s.tableJobs(ctx, inlet, ct.t.name, nil, 0)
		if err != nil {
			return err
		}
		jobs = append(jobs, tjobs...)
	}
	if err := s.runJobs(ctx, s.snapshotParallelism(), jobs); err != nil {
		return fmt.Errorf("sqlserver cdc: initial snapshot: %w", err)
	}
	return inlet.flush()
}

// snapshotParallelism is how many shards a CDC snapshot reads at once:
// max_conns when set, else defaultSnapshotParallelism.
func (s *Source) snapshotParallelism() int {
	if s.maxConns > 0 {
		return s.maxConns
	}
	return defaultSnapshotParallelism
}

// snapshotInlet opens a writer per shard on parts after the change writer's
// part 0, and keeps them so the snapshot can flush them before replay.
type snapshotInlet struct {
	run     *cdcRun
	mu      sync.Mutex
	writers []arrowbatch.RowWriter
}

func (i *snapshotInlet) Builder(resource string, part int, schema rowmodel.Schema) (arrowbatch.RowWriter, error) {
	if _, ok := i.run.byName[resource]; !ok {
		return nil, fmt.Errorf("snapshot of untracked table %q", resource)
	}
	w, err := i.run.sink.Builder(resource, part+1, schema)
	if err != nil {
		return nil, err
	}
	i.mu.Lock()
	i.writers = append(i.writers, w)
	i.mu.Unlock()
	return snapshotWriter{w}, nil
}

func (i *snapshotInlet) flush() error {
	for _, w := range i.writers {
		if err := w.Flush(); err != nil {
			return fmt.Errorf("sqlserver cdc: flush initial snapshot: %w", err)
		}
	}
	return nil
}

// snapshotWriter drops the key a shard read stamps on each row: a snapshot
// row inside a change stream carries no position of its own.
type snapshotWriter struct{ arrowbatch.RowWriter }

func (w snapshotWriter) EndRow(rowmodel.Meta) error { return w.RowWriter.EndRow(rowmodel.Meta{}) }
