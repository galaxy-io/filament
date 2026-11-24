package sqlserver

// Incremental reads follow a durable watermark: a cursor column followed by
// the primary key, so rows sharing a cursor value order deterministically at
// batch boundaries. The first run backfills the table through keyset shards
// from a watermark taken before it starts; later runs read the rows after the
// watermark, up to a high mark taken when the run starts.
//
// A datetime cursor misses a row whose transaction commits with an older
// value after the high mark was read; a lookback re-reads a recent window for
// an upsert sink to fold. A rowversion cursor is exact: the high mark stays
// below MIN_ACTIVE_ROWVERSION(), so no open transaction can still commit
// beneath it.

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/checkpoint"
)

// PlanIncremental builds a resumable keyset backfill on the first run and a
// single watermark plan on later runs.
func (s *Source) PlanIncremental(ctx context.Context, resources []string, prev map[string]filament.Checkpoint, cursors map[string]filament.ResourceCursorConfig) (map[string]filament.Checkpoint, error) {
	if err := s.configureCursors(resources, cursors); err != nil {
		return nil, err
	}

	plan := make(map[string]filament.Checkpoint, len(resources))
	for _, name := range resources {
		t, err := s.tableMeta(ctx, name)
		if err != nil {
			return nil, err
		}
		if len(t.pks) == 0 {
			return nil, fmt.Errorf("incremental %q requires a primary key", name)
		}
		if !t.orderedKey() {
			return nil, fmt.Errorf("incremental %q has a sql_variant primary key column, which a watermark cannot order", name)
		}
		cursor, err := s.incrementalCursor(t)
		if err != nil {
			return nil, err
		}
		if isRowversion(cursor.decl()) && s.cursorLookbacks[name] > 0 {
			return nil, fmt.Errorf("incremental %q: a rowversion cursor is exact and takes no lookback", name)
		}
		cols, types := incrementalKey(cursor, t.pks)
		if old, ok := checkpoint.ParseKeyset(prev[name]); ok {
			if old.Mode == checkpoint.ModeIncremental && slices.Equal(old.Cols, cols) &&
				slices.Equal(old.Types, types) && len(old.Shards) == 1 {
				plan[name] = old.ToCheckpoint(name)
				continue
			}
			if old.Mode == checkpoint.ModeIncrementalBackfill {
				if promoted, valid := checkpoint.PromoteIncrementalBackfill(prev[name]); valid {
					target, _ := checkpoint.ParseKeyset(promoted)
					if slices.Equal(target.Cols, cols) && slices.Equal(target.Types, types) && len(old.Shards) > 0 {
						plan[name] = old.ToCheckpoint(name)
						continue
					}
				}
			}
		}
		keys := keyColumns(cols, types)
		fence, err := s.rowversionFence(ctx, keys)
		if err != nil {
			return nil, fmt.Errorf("incremental %q initial watermark: %w", name, err)
		}
		start, err := s.incrementalHigh(ctx, s.db, t, keys, fence)
		if err != nil {
			return nil, fmt.Errorf("incremental %q initial watermark: %w", name, err)
		}
		backfill, err := s.planKeyset(ctx, t)
		if err != nil {
			return nil, fmt.Errorf("incremental %q backfill plan: %w", name, err)
		}
		backfill = checkpoint.AsIncrementalBackfill(backfill, cols, types, start, s.cursorLookbacks[name])
		plan[name] = backfill.ToCheckpoint(name)
	}
	return plan, nil
}

// configureCursors applies this plan's cursor configuration to each planned
// resource.
func (s *Source) configureCursors(resources []string, cursors map[string]filament.ResourceCursorConfig) error {
	if s.cursorColumns == nil {
		s.cursorColumns = map[string]string{}
	}
	if s.cursorLookbacks == nil {
		s.cursorLookbacks = map[string]int{}
	}
	// Each planned resource takes its configuration from this call alone. A
	// resource without a cursor field, or not in cursors at all, selects its
	// cursor afresh rather than keep one an earlier call set.
	for _, resource := range resources {
		config := cursors[resource]
		if config.LookbackSeconds < 0 {
			return fmt.Errorf("incremental %q lookback must be non-negative", resource)
		}
		if config.Field != "" {
			s.cursorColumns[resource] = config.Field
		} else {
			delete(s.cursorColumns, resource)
		}
		s.cursorLookbacks[resource] = int(config.LookbackSeconds)
	}
	return nil
}

// incrementalKey is the watermark's key: the cursor, then the primary key
// columns other than the cursor, which order rows that share a cursor value.
// A cursor that is the whole primary key is a key of one column.
func incrementalKey(cursor column, pks []column) (cols, types []string) {
	cols, types = []string{cursor.name}, []string{cursor.keyDecl()}
	for _, c := range pks {
		if c.name != cursor.name {
			cols, types = append(cols, c.name), append(types, c.keyDecl())
		}
	}
	return cols, types
}

// incrementalCursor resolves the table's cursor column: the configured one,
// or the first conventional name that is an eligible type.
func (s *Source) incrementalCursor(t *table) (column, error) {
	var name string
	if configured := s.cursorColumns[t.name]; configured != "" {
		c, err := t.column(configured)
		if err != nil {
			return column{}, fmt.Errorf("incremental %q cursor: %w", t.name, err)
		}
		name = c.name
	}
	if name == "" {
		for _, candidate := range cursorNamePriority {
			for _, c := range t.cols {
				if strings.EqualFold(c.name, candidate) && cursorEligible(c) {
					name = c.name
					break
				}
			}
			if name != "" {
				break
			}
		}
	}
	if name == "" {
		for _, c := range t.cols {
			if isRowversion(c.decl()) {
				name = c.name
				break
			}
		}
	}
	if name == "" {
		return column{}, fmt.Errorf("incremental %q has no supported cursor column; configure the pipeline resource cursor", t.name)
	}
	c, err := t.column(name)
	if err != nil {
		return column{}, fmt.Errorf("incremental %q cursor: %w", t.name, err)
	}
	if !isCursorType(c.decl()) {
		return column{}, fmt.Errorf("incremental %q cursor column %q must be a datetime or rowversion column, got %q", t.name, c.name, c.decl())
	}
	if c.nullable {
		return column{}, fmt.Errorf("incremental %q cursor column %q must be NOT NULL", t.name, c.name)
	}
	return c, nil
}

// incrementalJob reads the rows after a table's watermark.
func (s *Source) incrementalJob(ctx context.Context, sink arrowbatch.Inlet, name string, cp filament.Checkpoint, limit int) (job, error) {
	ks, ok := checkpoint.ParseKeyset(cp)
	if !ok || ks.Mode != checkpoint.ModeIncremental || len(ks.Cols) == 0 || len(ks.Shards) != 1 {
		return nil, fmt.Errorf("incremental %q has no valid checkpoint plan", name)
	}
	t, err := s.tableMeta(ctx, name)
	if err != nil {
		return nil, err
	}
	keys := keyColumns(ks.Cols, ks.Types)
	lookback := s.cursorLookbacks[name]
	// Taken now, before the job's transaction fixes its snapshot.
	fence, err := s.rowversionFence(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("incremental %q high watermark: %w", name, err)
	}
	return func(ctx context.Context, q querier) error {
		high, err := s.incrementalHigh(ctx, q, t, keys, fence)
		if err != nil {
			return fmt.Errorf("incremental %q high watermark: %w", name, err)
		}
		if high == nil {
			return nil
		}
		var args []any
		clauses := []string{"t." + quoteIdent(keys[0].name) + " IS NOT NULL"}
		if low := ks.Shards[0].Key; len(low) == len(keys) {
			if lookback == 0 {
				clauses = append(clauses, tupleCmp(keys, ">", low, &args))
			} else {
				// A lookback re-reads from a cursor value, inclusive; one
				// before the type's first value re-reads everything.
				rewound, inRange, err := rewind(low[0], lookback, keys[0].decl)
				if err != nil {
					return fmt.Errorf("incremental %q lookback: %w", name, err)
				}
				if inRange {
					clauses = append(clauses, tupleCmp(keys[:1], ">=", []string{rewound}, &args))
				}
			}
		}
		clauses = append(clauses, tupleCmp(keys, "<=", high, &args))
		r := &rowReader{
			schema: t.schema, limit: limit, keyNames: ks.Cols,
			open: builder(sink, name, 0, t.schema),
		}
		query := fmt.Sprintf("SELECT %s%s FROM %s t WHERE %s ORDER BY %s",
			top(limit), t.selectList, t.qualified, strings.Join(clauses, " AND "), keyOrder(keys, ""))
		if err := readRows(ctx, q, query, args, r); err != nil {
			return fmt.Errorf("incremental %q: %w", name, err)
		}
		return nil
	}, nil
}

// incrementalHigh reads the greatest (cursor, key) tuple a read may include,
// nil for an empty table. A rowversion cursor stays below fence.
func (s *Source) incrementalHigh(ctx context.Context, q querier, t *table, keys []keyColumn, fence []byte) ([]string, error) {
	cols := make([]string, len(keys))
	values := make([]string, len(keys))
	for i, k := range keys {
		cols[i] = "t." + quoteIdent(k.name)
		values[i] = keyValue(k.decl, cols[i])
	}
	where := cols[0] + " IS NOT NULL"
	var args []any
	if fence != nil {
		where += " AND " + cols[0] + " < @p1"
		args = append(args, fence)
	}
	query := fmt.Sprintf("SELECT TOP (1) %s FROM %s t WHERE %s ORDER BY %s",
		strings.Join(values, ", "), t.qualified, where, keyOrder(keys, " DESC"))
	rows, err := queryKeys(ctx, q, query, args...)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

// rowversionFence reads MIN_ACTIVE_ROWVERSION() for a rowversion cursor, and
// nil for any other. Every rowversion below it belongs to a transaction that
// had ended when it was read, so a snapshot fixed after that sees every row
// below it. Read inside the snapshot instead, it can move past a transaction
// that commits after the snapshot was fixed, whose rows the read then skips.
func (s *Source) rowversionFence(ctx context.Context, keys []keyColumn) ([]byte, error) {
	if !isRowversion(keys[0].decl) {
		return nil, nil
	}
	var fence []byte
	if err := s.db.QueryRowContext(ctx, "SELECT MIN_ACTIVE_ROWVERSION()").Scan(&fence); err != nil {
		return nil, fmt.Errorf("active rowversion: %w", err)
	}
	return fence, nil
}

// rewind moves a datetime cursor's checkpoint text back by seconds, keeping
// its form: the fractional digits and the UTC suffix of a datetimeoffset. It
// reports false when the result falls before the first value decl can hold,
// which no cast to decl accepts.
func rewind(text string, seconds int, decl string) (string, bool, error) {
	body, utc := strings.CutSuffix(text, "Z")
	digits := 0
	if i := strings.IndexByte(body, '.'); i >= 0 {
		digits = len(body) - i - 1
	}
	layout := "2006-01-02T15:04:05"
	if digits > 0 {
		layout += "." + strings.Repeat("0", digits)
	}
	ts, err := time.Parse(layout, body)
	if err != nil {
		return "", false, fmt.Errorf("cursor %q is not a datetime", text)
	}
	back := ts.Add(-time.Duration(seconds) * time.Second)
	if back.Before(minTime(decl)) {
		return "", false, nil
	}
	out := back.Format(layout)
	if utc {
		out += "Z"
	}
	return out, true, nil
}

// minTime is the first value a datetime type holds.
func minTime(decl string) time.Time {
	switch decl {
	case "datetime":
		return time.Date(1753, 1, 1, 0, 0, 0, 0, time.UTC)
	case "smalldatetime":
		return time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	default: // datetime2, datetimeoffset
		return time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	}
}

func keyColumns(names, types []string) []keyColumn {
	keys := make([]keyColumn, len(names))
	for i := range names {
		keys[i] = keyColumn{name: names[i], decl: typeAt(types, i)}
	}
	return keys
}
