package postgres

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
)

var cursorNamePriority = []string{
	"updated_at", "modified_at", "last_modified_at", "last_updated_at", "updated", "modified",
}

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
		if old, ok := checkpoint.ParseKeyset(prev[table]); ok && old.Mode == checkpoint.ModeIncremental &&
			slices.Equal(old.Cols, cols) && slices.Equal(old.Types, types) && len(old.Shards) == 1 {
			plan[table] = old.ToCheckpoint(table)
			continue
		}
		plan[table] = checkpoint.KeysetCheckpoint{
			Mode: checkpoint.ModeIncremental, Cols: cols, Types: types,
			Shards: []checkpoint.KeysetShard{{}},
		}.ToCheckpoint(table)
	}
	return plan, nil
}

func (s *Source) incrementalCursor(ctx context.Context, table string) (pkColumn, error) {
	schema, err := s.Schema(ctx, table)
	if err != nil {
		return pkColumn{}, fmt.Errorf("incremental %q schema: %w", table, err)
	}
	byName := make(map[string]filament.SchemaField, len(schema.Fields))
	for _, field := range schema.Fields {
		byName[strings.ToLower(field.Name)] = field
	}
	name := s.cursorColumns[table]
	if name == "" {
		for _, candidate := range cursorNamePriority {
			if field, ok := byName[candidate]; ok && isTimestampType(field.Native) {
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

func (s *Source) extractIncremental(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts, plans map[string]filament.Checkpoint) error {
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

func (s *Source) extractIncrementalTable(ctx context.Context, sink filament.RecordSink, table string, ks checkpoint.KeysetCheckpoint, limit int) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("incremental %q begin snapshot: %w", table, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	cols := make([]pkColumn, len(ks.Cols))
	for i := range ks.Cols {
		cols[i] = pkColumn{name: ks.Cols[i], typ: typeAt(ks.Types, i)}
	}
	cursor, pks := cols[0], cols[1:]
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
		n, last, err := s.readIncrementalPageWithKey(ctx, tx, sink, table, qualified, cursor, pks, where, args, s.pageSize, remaining(limit, emitted))
		if err != nil {
			return err
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

func (s *Source) readIncrementalPageWithKey(ctx context.Context, qx querier, sink filament.RecordSink, table, qualified string, cursor pkColumn, pks []pkColumn, where string, args []any, pageSize, limit int) (int, []string, error) {
	keyCols := append([]pkColumn{cursor}, pks...)
	keys := make([]string, len(keyCols))
	projections := make([]string, len(keyCols))
	order := make([]string, len(keyCols))
	for i, col := range keyCols {
		ident := "t." + pgx.Identifier{col.name}.Sanitize()
		projections[i], order[i] = ident+"::text", ident
	}
	q := fmt.Sprintf("SELECT %s AS id, to_jsonb(t)::text AS data, %s FROM %s t WHERE %s ORDER BY %s",
		keysetIDExpr(pks), strings.Join(projections, ", "), qualified, where, strings.Join(order, ", "))
	if pageSize > 0 {
		q += fmt.Sprintf(" LIMIT %d", pageSize)
	}
	rows, err := qx.Query(ctx, q, args...)
	if err != nil {
		return 0, nil, fmt.Errorf("incremental %q query: %w", table, err)
	}
	defer rows.Close()
	var id string
	var data []byte
	dest := make([]any, 2, 2+len(keys))
	dest[0], dest[1] = &id, &data
	for i := range keys {
		dest = append(dest, &keys[i])
	}
	n := 0
	var last []string
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return n, last, err
		}
		rec := filament.NewRecord(table, id, append([]byte(nil), data...))
		if keys[0] != "" {
			rec.Key = append([]string(nil), keys...)
		}
		if err := sink.Push(rec); err != nil {
			return n, last, err
		}
		last = append(last[:0], keys...)
		n++
		if limit > 0 && n >= limit {
			break
		}
	}
	return n, last, rows.Err()
}
