package mysql

// Incremental reads use a durable timestamp watermark followed by the complete
// primary key. The compound key makes rows sharing a timestamp deterministic at
// page and batch boundaries. A bounded lookback deliberately re-reads a recent
// window so late commits are folded safely by an upsert sink.

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/rowmodel"
)

const defaultIncrementalLookbackSeconds = 0

// PlanIncremental builds a resumable initial PK backfill on the first run and a
// single compound watermark plan on subsequent runs.
func (s *Source) PlanIncremental(ctx context.Context, resources []string, prev map[string]filament.Checkpoint, cursors map[string]filament.ResourceCursorConfig) (map[string]filament.Checkpoint, error) {
	if s.cursorColumns == nil {
		s.cursorColumns = map[string]string{}
	}
	if s.cursorLookbacks == nil {
		s.cursorLookbacks = map[string]int{}
	}
	for resource, config := range cursors {
		if config.Field != "" {
			s.cursorColumns[resource] = config.Field
		}
		if config.LookbackSeconds < 0 {
			return nil, fmt.Errorf("incremental %q lookback must be non-negative", resource)
		}
		s.cursorLookbacks[resource] = int(config.LookbackSeconds)
	}

	plan := make(map[string]filament.Checkpoint, len(resources))
	for _, table := range resources {
		if _, configured := s.cursorLookbacks[table]; !configured {
			s.cursorLookbacks[table] = defaultIncrementalLookbackSeconds
		}
		_, pks, err := s.tableMeta(ctx, table)
		if err != nil {
			return nil, fmt.Errorf("incremental %q primary key: %w", table, err)
		}
		if len(pks) == 0 {
			return nil, fmt.Errorf("incremental %q requires a primary key", table)
		}
		cursor, err := s.incrementalCursor(ctx, table)
		if err != nil {
			return nil, err
		}
		cols := append([]string{cursor.name}, pkNames(pks)...)
		types := append([]string{cursor.typ}, pkTypes(pks)...)
		if old, ok := checkpoint.ParseKeyset(prev[table]); ok {
			if old.Mode == checkpoint.ModeIncremental && slices.Equal(old.Cols, cols) &&
				slices.Equal(old.Types, types) && len(old.Shards) == 1 {
				plan[table] = old.ToCheckpoint(table)
				continue
			}
			if old.Mode == checkpoint.ModeIncrementalBackfill {
				if promoted, valid := checkpoint.PromoteIncrementalBackfill(prev[table]); valid {
					target, _ := checkpoint.ParseKeyset(promoted)
					if slices.Equal(target.Cols, cols) && slices.Equal(target.Types, types) && len(old.Shards) > 0 {
						plan[table] = old.ToCheckpoint(table)
						continue
					}
				}
			}
		}

		qualified := quoteIdent(s.database) + "." + quoteIdent(table)
		start, err := s.incrementalHigh(ctx, s.db, qualified, append([]pkColumn{cursor}, pks...))
		if err != nil {
			return nil, fmt.Errorf("incremental %q initial watermark: %w", table, err)
		}
		backfill, err := s.planKeyset(ctx, table, qualified, pks)
		if err != nil {
			return nil, fmt.Errorf("incremental %q backfill plan: %w", table, err)
		}
		backfill = checkpoint.AsIncrementalBackfill(backfill, cols, types, start, s.cursorLookbacks[table])
		plan[table] = backfill.ToCheckpoint(table)
	}
	return plan, nil
}

func (s *Source) incrementalCursor(ctx context.Context, table string) (pkColumn, error) {
	schema, err := s.Schema(ctx, table)
	if err != nil {
		return pkColumn{}, fmt.Errorf("incremental %q schema: %w", table, err)
	}
	byName := make(map[string]rowmodel.Field, len(schema.Fields))
	for _, field := range schema.Fields {
		byName[strings.ToLower(field.Name)] = field
	}
	name := s.cursorColumns[table]
	if name == "" {
		for _, candidate := range cursorNamePriority {
			if field, ok := byName[candidate]; ok && isTimestampCursorType(field.Native) && !field.Nullable {
				name = field.Name
				break
			}
		}
	}
	if name == "" {
		return pkColumn{}, fmt.Errorf("incremental %q has no supported cursor column; configure the pipeline resource cursor", table)
	}
	field, ok := byName[strings.ToLower(name)]
	if !ok {
		return pkColumn{}, fmt.Errorf("incremental %q cursor column %q does not exist", table, name)
	}
	if !isTimestampCursorType(field.Native) {
		return pkColumn{}, fmt.Errorf("incremental %q cursor column %q must be a timestamp updated on every insert and update, got %q", table, field.Name, field.Native)
	}
	if field.Nullable {
		return pkColumn{}, fmt.Errorf("incremental %q cursor column %q must be NOT NULL", table, field.Name)
	}
	return pkColumn{name: field.Name, typ: field.Native}, nil
}

// extractIncremental runs each table in its own consistent-snapshot transaction.
func (s *Source) extractIncremental(ctx context.Context, sink arrowbatch.Inlet, opts filament.ExtractOpts, plans map[string]filament.Checkpoint) error {
	jobs := make([]func(context.Context, querier) error, 0, len(opts.Resources))
	for _, table := range opts.Resources {
		ks, ok := checkpoint.ParseKeyset(plans[table])
		if !ok || ks.Mode != checkpoint.ModeIncremental || len(ks.Cols) < 2 || len(ks.Shards) != 1 {
			return fmt.Errorf("incremental %q has no valid checkpoint plan", table)
		}
		jobs = append(jobs, func(ctx context.Context, q querier) error {
			return s.extractIncrementalTable(ctx, sink, q, table, ks, opts.Limit)
		})
	}
	return s.runConcurrent(ctx, opts.Parallelism, jobs)
}

func (s *Source) extractIncrementalTable(ctx context.Context, sink arrowbatch.Inlet, q querier, table string, ks checkpoint.KeysetCheckpoint, limit int) error {
	cols := make([]pkColumn, len(ks.Cols))
	for i := range ks.Cols {
		cols[i] = pkColumn{name: ks.Cols[i], typ: typeAt(ks.Types, i)}
	}
	cursor, pks := cols[0], cols[1:]
	dec, err := s.decoderFor(ctx, table)
	if err != nil {
		return err
	}
	idx, err := dec.indexOf(append([]string{cursor.name}, pkNames(pks)...))
	if err != nil {
		return err
	}
	w, err := sink.Builder(table, 0, dec.schema)
	if err != nil {
		return err
	}
	qualified := quoteIdent(s.database) + "." + quoteIdent(table)
	high, err := s.incrementalHigh(ctx, q, qualified, cols)
	if err != nil {
		return fmt.Errorf("incremental %q high watermark: %w", table, err)
	}
	if high == nil {
		return nil
	}
	low := ks.Shards[0].Key
	if len(low) == len(cols) && s.cursorLookbacks[table] > 0 {
		low, err = rewindIncrementalLow(ctx, q, low[0], s.cursorLookbacks[table])
		if err != nil {
			return fmt.Errorf("incremental %q apply lookback: %w", table, err)
		}
	}
	emitted := 0
	for {
		where, args := incrementalBounds(cols, low, high)
		query := incrementalPageSQL(qualified, dec.selectList, cols, where, s.pageSize)
		rows, err := q.QueryContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("incremental %q: %w", table, err)
		}
		n, last, appendErr := dec.appendRows(rows, w, idx, cols, remaining(limit, emitted))
		_ = rows.Close()
		if appendErr != nil {
			return fmt.Errorf("incremental %q: %w", table, appendErr)
		}
		emitted += n
		if n == 0 || n < s.pageSize || (limit > 0 && emitted >= limit) {
			return nil
		}
		low = last
	}
}

func (s *Source) incrementalHigh(ctx context.Context, q querier, qualified string, cols []pkColumn) ([]string, error) {
	selects := make([]string, len(cols))
	order := make([]string, len(cols))
	for i, col := range cols {
		ident := "t." + quoteIdent(col.name)
		selects[i], order[i] = ident, ident+" DESC"
	}
	query := fmt.Sprintf("SELECT %s FROM %s t WHERE t.%s IS NOT NULL ORDER BY %s LIMIT 1",
		strings.Join(selects, ", "), qualified, quoteIdent(cols[0].name), strings.Join(order, ", "))
	return queryCheckpointValues(ctx, q, query, cols)
}

func queryCheckpointValues(ctx context.Context, q querier, query string, cols []pkColumn, args ...any) ([]string, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	raw := make([]sql.RawBytes, len(cols))
	dest := make([]any, len(raw))
	for i := range raw {
		dest[i] = &raw[i]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	values := make([]string, len(cols))
	for i := range raw {
		values[i] = cols[i].checkpointValue(raw[i])
	}
	return values, rows.Err()
}

func rewindIncrementalLow(ctx context.Context, q querier, value string, seconds int) ([]string, error) {
	query := "SELECT CAST(DATE_SUB(CAST(? AS DATETIME(6)), INTERVAL ? SECOND) AS CHAR)"
	return queryStrings(ctx, q, query, 1, value, seconds)
}

func queryStrings(ctx context.Context, q querier, query string, count int, args ...any) ([]string, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	raw := make([]sql.RawBytes, count)
	dest := make([]any, count)
	for i := range raw {
		dest[i] = &raw[i]
	}
	if err := rows.Scan(dest...); err != nil {
		return nil, err
	}
	values := make([]string, count)
	for i := range raw {
		values[i] = string(raw[i])
	}
	return values, rows.Err()
}

func incrementalBounds(cols []pkColumn, low, high []string) (string, []any) {
	clauses := []string{"t." + quoteIdent(cols[0].name) + " IS NOT NULL"}
	var args []any
	if len(low) == len(cols) {
		clause, values := tupleCmp(cols, ">", low)
		clauses, args = append(clauses, clause), append(args, values...)
	} else if len(low) == 1 {
		ident := "t." + quoteIdent(cols[0].name)
		clauses = append(clauses, ident+" >= "+castExpr(cols[0].typ))
		args = append(args, low[0])
	}
	clause, values := tupleCmp(cols, "<=", high)
	clauses, args = append(clauses, clause), append(args, values...)
	return strings.Join(clauses, " AND "), args
}

func incrementalPageSQL(qualified, selectList string, cols []pkColumn, where string, pageSize int) string {
	order := make([]string, len(cols))
	for i, col := range cols {
		order[i] = "t." + quoteIdent(col.name)
	}
	query := fmt.Sprintf("SELECT %s FROM %s t WHERE %s ORDER BY %s", selectList, qualified, where, strings.Join(order, ", "))
	if pageSize > 0 {
		query += fmt.Sprintf(" LIMIT %d", pageSize)
	}
	return query
}
