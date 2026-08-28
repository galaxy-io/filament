package postgres

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/rowmodel"
)

var cursorNamePriority = []string{
	"updated_at", "modified_at", "last_modified_at", "last_updated_at", "updated", "modified",
}

const defaultIncrementalLookbackSeconds = 300

// CursorColumns reports every table column and ranks supported conventional
// watermark names.
func (s *Source) CursorColumns(ctx context.Context, table string) ([]filament.CursorColumn, error) {
	schema, err := s.Schema(ctx, table)
	if err != nil {
		return nil, err
	}
	pk := make(map[string]bool, len(schema.PrimaryKey))
	for _, name := range schema.PrimaryKey {
		pk[name] = true
	}
	priority := make(map[string]int, len(cursorNamePriority))
	for i, name := range cursorNamePriority {
		priority[name] = i + 1
	}
	explicit := s.cursorColumns[table]
	out := make([]filament.CursorColumn, 0, len(schema.Fields))
	best := 0
	for _, field := range schema.Fields {
		rank := priority[strings.ToLower(field.Name)]
		if explicit != "" && strings.EqualFold(explicit, field.Name) {
			rank = 1
		}
		// Cross-run upserts require a value that advances when a row changes.
		// Sortable IDs/text are useful keysets but unsafe update watermarks, so
		// incremental cursors are deliberately limited to timestamps.
		eligible := isTimestampType(field.Native) && !field.Nullable
		var warnings []string
		if isTimestampType(field.Native) && field.Nullable {
			warnings = append(warnings, "Durable cursor columns must be NOT NULL")
		}
		if eligible && priority[strings.ToLower(field.Name)] == 0 {
			warnings = append(warnings, "Use only if this timestamp advances monotonically on every insert and update")
		}
		out = append(out, filament.CursorColumn{
			SchemaField: field, PrimaryKey: pk[field.Name], Eligible: eligible, Rank: rank,
			Configurable: true, SupportsLookback: eligible, Warning: strings.Join(warnings, "; "),
		})
		if eligible && rank > 0 && (best == 0 || rank < best) {
			best = rank
		}
	}
	for i := range out {
		out[i].Recommended = out[i].Eligible && out[i].Rank > 0 && out[i].Rank == best
	}
	return out, nil
}

// PlanIncremental builds a single compound watermark per table. The source
// cursor is followed by the complete primary key so equal cursor values are
// deterministic and cannot skip rows at a batch boundary.
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
		if config.LookbackSeconds > 0 {
			s.cursorLookbacks[resource] = int(config.LookbackSeconds)
		}
	}
	plan := make(map[string]filament.Checkpoint, len(resources))
	for _, table := range resources {
		if _, configured := s.cursorLookbacks[table]; !configured {
			s.cursorLookbacks[table] = defaultIncrementalLookbackSeconds
		}
		pks, err := s.lookupPrimaryKey(ctx, s.schema, table)
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
		if s.cursorLookbacks[table] > 0 && !isTimestampType(cursor.typ) {
			return nil, fmt.Errorf("incremental %q lookback requires a timestamp cursor, got %q", table, cursor.typ)
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
		qualified := pgx.Identifier{s.schema, table}.Sanitize()
		start, err := s.incrementalHigh(ctx, s.pool, qualified, append([]pkColumn{cursor}, pks...))
		if err != nil {
			return nil, fmt.Errorf("incremental %q initial watermark: %w", table, err)
		}
		backfill, err := s.planKeyset(ctx, table, pks)
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
			if field, ok := byName[candidate]; ok && isTimestampType(field.Native) && !field.Nullable {
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
	if !isTimestampType(field.Native) {
		return pkColumn{}, fmt.Errorf("incremental %q cursor column %q must be a timestamp updated on every insert and update, got %q", table, field.Name, field.Native)
	}
	if field.Nullable {
		return pkColumn{}, fmt.Errorf("incremental %q cursor column %q must be NOT NULL", table, field.Name)
	}
	return pkColumn{name: field.Name, typ: field.Native}, nil
}

func isTimestampType(native string) bool {
	t := strings.ToLower(strings.TrimSpace(native))
	if strings.HasPrefix(t, "timestamp(") {
		if end := strings.IndexByte(t, ')'); end >= 0 {
			t = "timestamp" + t[end+1:]
		}
	}
	if strings.HasPrefix(t, "timestamptz(") {
		if end := strings.IndexByte(t, ')'); end >= 0 {
			t = "timestamptz" + t[end+1:]
		}
	}
	return t == "timestamp" || t == "timestamptz" || t == "timestamp without time zone" || t == "timestamp with time zone"
}

func (s *Source) extractIncremental(ctx context.Context, sink arrowbatch.Inlet, opts filament.ExtractOpts, plans map[string]filament.Checkpoint) error {
	for _, table := range opts.Resources {
		ks, ok := checkpoint.ParseKeyset(plans[table])
		if !ok || ks.Mode != checkpoint.ModeIncremental || len(ks.Cols) < 2 || len(ks.Shards) != 1 {
			return fmt.Errorf("incremental %q has no valid checkpoint plan", table)
		}
		if err := s.extractIncrementalTable(ctx, sink, table, ks, opts.Limit); err != nil {
			return err
		}
	}
	return nil
}

func (s *Source) extractIncrementalTable(ctx context.Context, sink arrowbatch.Inlet, table string, ks checkpoint.KeysetCheckpoint, limit int) error {
	cols := make([]pkColumn, len(ks.Cols))
	for i := range ks.Cols {
		cols[i] = pkColumn{name: ks.Cols[i], typ: typeAt(ks.Types, i)}
	}
	cursor, pks := cols[0], cols[1:]
	dec, err := s.decoderFor(ctx, table, pkNames(pks))
	if err != nil {
		return err
	}
	cursorIdx := dec.index(cursor.name)
	if cursorIdx < 0 {
		return fmt.Errorf("incremental %q cursor column %q not in table", table, cursor.name)
	}
	keyIdx := append([]int{cursorIdx}, dec.pkIdx...) // each row's key is [watermark, pk...]
	w, err := sink.Builder(table, 0, dec.schema)
	if err != nil {
		return err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("incremental %q begin snapshot: %w", table, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qualified := pgx.Identifier{s.schema, table}.Sanitize()
	high, err := s.incrementalHigh(ctx, tx, qualified, cols)
	if err != nil {
		return fmt.Errorf("incremental %q high watermark: %w", table, err)
	}
	emitted := 0
	if high == nil {
		return tx.Commit(ctx)
	}
	low := ks.Shards[0].Key
	if len(low) == len(cols) && s.cursorLookbacks[table] > 0 {
		var rewound string
		q := fmt.Sprintf("SELECT ($1::%s - make_interval(secs => $2))::text", cursor.typ)
		if err := tx.QueryRow(ctx, q, low[0], s.cursorLookbacks[table]).Scan(&rewound); err != nil {
			return fmt.Errorf("incremental %q apply lookback: %w", table, err)
		}
		low = []string{rewound}
	}
	for {
		where, args := incrementalBounds(cols, low, high)
		sql := incrementalPageSQL(qualified, dec.selectList, cols, where, s.pageSize)
		n, last, err := s.appendPage(ctx, tx, sql, w, dec, rowmodel.Meta{}, keyIdx, remaining(limit, emitted), args...)
		if err != nil {
			return fmt.Errorf("incremental %q: %w", table, err)
		}
		emitted += n
		if n == 0 || n < s.pageSize || (limit > 0 && emitted >= limit) {
			return tx.Commit(ctx)
		}
		low = last
	}
}

type incrementalQuerier interface {
	querier
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (s *Source) incrementalHigh(ctx context.Context, qx incrementalQuerier, qualified string, cols []pkColumn) ([]string, error) {
	selects := make([]string, len(cols))
	order := make([]string, len(cols))
	for i, col := range cols {
		ident := "t." + pgx.Identifier{col.name}.Sanitize()
		selects[i], order[i] = ident+"::text", ident+" DESC"
	}
	q := fmt.Sprintf("SELECT %s FROM %s t WHERE %s IS NOT NULL ORDER BY %s LIMIT 1",
		strings.Join(selects, ", "), qualified, "t."+pgx.Identifier{cols[0].name}.Sanitize(), strings.Join(order, ", "))
	values := make([]string, len(cols))
	dest := make([]any, len(cols))
	for i := range values {
		dest[i] = &values[i]
	}
	err := qx.QueryRow(ctx, q).Scan(dest...)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return values, err
}

func incrementalBounds(cols []pkColumn, low, high []string) (string, []any) {
	clauses := []string{"t." + pgx.Identifier{cols[0].name}.Sanitize() + " IS NOT NULL"}
	var args []any
	n := 1
	if len(low) == len(cols) {
		clause, a, next := tupleCmp(cols, ">", low, n)
		clauses, args, n = append(clauses, clause), append(args, a...), next
	} else if len(low) == 1 {
		ident := "t." + pgx.Identifier{cols[0].name}.Sanitize()
		clauses = append(clauses, fmt.Sprintf("%s >= $%d::%s", ident, n, cols[0].typ))
		args = append(args, low[0])
		n++
	}
	clause, a, _ := tupleCmp(cols, "<=", high, n)
	clauses, args = append(clauses, clause), append(args, a...)
	return strings.Join(clauses, " AND "), args
}

func remaining(limit, emitted int) int {
	if limit <= 0 {
		return 0
	}
	return max(limit-emitted, 0)
}

// incrementalPageSQL selects the decoder's projection ordered by the cursor then the
// key, so each page is a bounded index range scan.
func incrementalPageSQL(qualified, selectList string, cols []pkColumn, where string, pageSize int) string {
	order := make([]string, len(cols))
	for i, col := range cols {
		order[i] = "t." + pgx.Identifier{col.name}.Sanitize()
	}
	q := fmt.Sprintf("SELECT %s FROM %s t WHERE %s ORDER BY %s", selectList, qualified, where, strings.Join(order, ", "))
	if pageSize > 0 {
		q += fmt.Sprintf(" LIMIT %d", pageSize)
	}
	return q
}
