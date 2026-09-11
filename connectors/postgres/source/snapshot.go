package postgres

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// Extract plans every requested table into shards and pages them out through the
// sink. With Parallelism > 1 the shards are read concurrently (bounded by
// Parallelism) into the shared sink under one exported snapshot; paging within a
// shard stays sequential. The first shard error cancels the rest and is returned.
func (s *Source) Extract(ctx context.Context, sink arrowbatch.Inlet, opts filament.ExtractOpts) error {
	shards, err := s.planShards(ctx, opts.Resources, opts.Parallelism)
	if err != nil {
		return err
	}

	// Sequential path: one worker, no cross-shard concurrency, so no snapshot to
	// coordinate — read each shard in turn straight off the pool.
	if opts.Parallelism <= 1 || len(shards) <= 1 {
		for _, sh := range shards {
			if err := s.extractShard(ctx, sink, s.pool, sh, opts.Limit); err != nil {
				return fmt.Errorf("extract %q: %w", sh.table, err)
			}
		}
		return nil
	}

	// Concurrent path: pin one consistent snapshot for every shard, then fan out.
	snap, err := s.openSnapshot(ctx)
	if err != nil {
		return err
	}
	defer snap.close(ctx)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, opts.Parallelism)
	var wg sync.WaitGroup
	var errOnce sync.Once
	var firstErr error

	for _, sh := range shards {
		wg.Add(1)
		go func(sh shard) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return // a sibling already failed; stop launching work
			}
			if err := s.extractShardSnapshot(ctx, sink, snap, sh, opts.Limit); err != nil {
				errOnce.Do(func() {
					firstErr = fmt.Errorf("extract %q: %w", sh.table, err)
					cancel() // stop the other readers
				})
			}
		}(sh)
	}
	wg.Wait()
	return firstErr
}

// shard is one independently-readable slice of a table's heap: the half-open heap
// block range [loBlock, hiBlock). The reader walks it in fixed windows of windowBlocks
// blocks, so every query is a bounded TID range scan (no whole-table scan, no sort).
// qualified and dec are precomputed so the window loop is pure string formatting.
type shard struct {
	table        string      // raw table name, used as the record resource
	part         int         // stable per-table shard identity for the pipeline builder
	qualified    string      // sanitized "schema"."table" for the FROM clause
	dec          *rowDecoder // row decoder, shared read-only by the table's shards
	loBlock      int         // inclusive first heap block
	hiBlock      int         // exclusive last heap block
	windowBlocks int         // blocks read per query, sized to ≈ page_size rows
}

// planShards turns each requested table into one or more shards. A table is split
// only when sharding is enabled, the run is parallel, and the table is larger than
// shard_pages; otherwise it is a single shard covering the whole heap. Every shard
// is bounded by the table's current block count — an unbounded ctid scan re-reads to
// end-of-table on each page, so it is avoided.
func (s *Source) planShards(ctx context.Context, tables []string, parallelism int) ([]shard, error) {
	var shards []shard
	for _, table := range tables {
		pks, err := s.lookupPrimaryKey(ctx, s.schema, table)
		if err != nil {
			return nil, fmt.Errorf("lookup pk %q: %w", table, err)
		}
		qualified := pgx.Identifier{s.schema, table}.Sanitize()
		dec, err := s.decoderFor(ctx, table, pkNames(pks))
		if err != nil {
			return nil, err
		}

		pages, err := s.lookupPages(ctx, qualified)
		if err != nil {
			return nil, fmt.Errorf("lookup size %q: %w", table, err)
		}
		window, err := s.windowBlocks(ctx, qualified)
		if err != nil {
			return nil, fmt.Errorf("size window %q: %w", table, err)
		}
		k := shardCount(pages, parallelism, s.shardPages)
		for i := range k {
			lo := i * pages / k
			hi := (i + 1) * pages / k
			if i == k-1 {
				hi = pages // exact end; rows live in blocks [0, pages)
			}
			shards = append(shards, shard{
				table: table, part: i, qualified: qualified, dec: dec,
				loBlock: lo, hiBlock: hi, windowBlocks: window,
			})
		}
	}
	return shards, nil
}

// shardCount decides how many shards a table of `pages` heap blocks becomes.
func shardCount(pages, parallelism, shardPages int) int {
	if shardPages <= 0 || parallelism <= 1 || pages <= shardPages {
		return 1
	}
	return min((pages+shardPages-1)/shardPages, maxShardsPerTable)
}

// quoteLiteral renders s as a single-quoted SQL string literal. Used only for
// catalog-sourced column names that must be inlined into a SELECT projection.
func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// querier is the subset of pgx used by the page loop, satisfied by both the pool
// and a snapshot transaction.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// extractShard reads one shard's block windows into a fresh writer for the table.
// Each window is a half-open block range bound on both ends, so the query is a
// bounded TID range scan reading only that window — never the rest of the table.
// limit > 0 stops the shard after that many rows (best-effort per shard when
// split).
func (s *Source) extractShard(ctx context.Context, sink arrowbatch.Inlet, q querier, sh shard, limit int) error {
	w, err := sink.Builder(sh.table, sh.part, sh.dec.schema)
	if err != nil {
		return err
	}
	_, err = s.readBlocks(ctx, w, q, sh, "", nil, rowmodel.Meta{}, limit)
	return err
}

// readBlocks appends the shard's block windows into w, one bounded TID range scan
// per window. filter is an extra AND clause on t (or empty) with its args after
// the two ctid bounds. Returns the rows appended; limit > 0 stops after that many.
func (s *Source) readBlocks(ctx context.Context, w arrowbatch.RowWriter, q querier, sh shard, filter string, extra []any, meta rowmodel.Meta, limit int) (int, error) {
	sql := ctidWindowSQL(sh, filter)
	emitted := 0
	for b := sh.loBlock; b < sh.hiBlock; b += sh.windowBlocks {
		hi := min(b+sh.windowBlocks, sh.hiBlock)
		args := append([]any{fmt.Sprintf("(%d,0)", b), fmt.Sprintf("(%d,0)", hi)}, extra...)
		n, _, err := s.appendPage(ctx, q, sql, w, sh.dec, meta, nil, remaining(limit, emitted), args...)
		if err != nil {
			return emitted, err
		}
		emitted += n
		if limit > 0 && emitted >= limit {
			return emitted, nil
		}
	}
	return emitted, nil
}

// ctidWindowSQL builds one shard's block-window query, with filter (an extra AND
// clause on t, or empty) appended to the ctid range.
func ctidWindowSQL(sh shard, filter string) string {
	return fmt.Sprintf(
		"SELECT %[1]s FROM %[2]s t WHERE t.ctid >= $1::tid AND t.ctid < $2::tid%[3]s",
		sh.dec.selectList, sh.qualified, filter)
}

// binaryResults asks the server for binary wire format on every result column.
// Columns projected ::text are byte-identical in either format.
var binaryResults = pgx.QueryResultFormats{pgx.BinaryFormatCode}

// appendPage runs one query in binary result format and appends every row into
// w. keyIdx names the columns whose text forms become each row's RowMeta.Key
// (the keyset or incremental cursor); a nil keyIdx ends rows with meta as given.
// limit > 0 stops after that many rows. Returns the rows appended and the last
// row's key.
func (s *Source) appendPage(ctx context.Context, q querier, sql string, w arrowbatch.RowWriter, dec *rowDecoder, meta rowmodel.Meta, keyIdx []int, limit int, args ...any) (int, []string, error) {
	rows, err := q.Query(ctx, sql, append([]any{binaryResults}, args...)...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	n := 0
	var last []string
	for rows.Next() {
		raw := rows.RawValues()
		if keyIdx != nil {
			if meta.Key, err = dec.texts(raw, keyIdx); err != nil {
				return n, last, err
			}
		}
		if err := dec.appendRow(w, raw); err != nil {
			return n, last, fmt.Errorf("read row: %w", err)
		}
		if err := w.EndRow(meta); err != nil {
			return n, last, err
		}
		last = meta.Key
		n++
		if limit > 0 && n >= limit {
			break
		}
	}
	return n, last, rows.Err()
}

// snapshot holds an exported-snapshot transaction open for a run's lifetime. Its
// connection stays checked out of the pool so the snapshot remains importable by
// every shard transaction; close releases both.
type snapshot struct {
	conn *pgxpool.Conn
	tx   pgx.Tx
	id   string
}

// openSnapshot begins a read-only repeatable-read transaction and exports its
// snapshot id. Shard transactions import it via SET TRANSACTION SNAPSHOT so all
// shards read one consistent point-in-time.
func (s *Source) openSnapshot(ctx context.Context) (*snapshot, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire snapshot conn: %w", err)
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		conn.Release()
		return nil, fmt.Errorf("begin snapshot tx: %w", err)
	}
	var id string
	if err := tx.QueryRow(ctx, "SELECT pg_export_snapshot()").Scan(&id); err != nil {
		_ = tx.Rollback(ctx)
		conn.Release()
		return nil, fmt.Errorf("export snapshot: %w", err)
	}
	return &snapshot{conn: conn, tx: tx, id: id}, nil
}

func (sn *snapshot) close(ctx context.Context) {
	if sn == nil {
		return
	}
	_ = sn.tx.Rollback(ctx)
	sn.conn.Release()
}

// extractShardSnapshot reads one shard inside its own transaction that imports the
// run's exported snapshot, then delegates to the shared page loop.
func (s *Source) extractShardSnapshot(ctx context.Context, sink arrowbatch.Inlet, snap *snapshot, sh shard, limit int) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire shard conn: %w", err)
	}
	defer conn.Release()

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("begin shard tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// The snapshot id is server-generated; SET TRANSACTION SNAPSHOT takes no bind
	// parameter, so it is quoted and inlined.
	if _, err := tx.Exec(ctx, "SET TRANSACTION SNAPSHOT "+quoteLiteral(snap.id)); err != nil {
		return fmt.Errorf("import snapshot: %w", err)
	}
	err = s.extractShard(ctx, sink, tx, sh, limit)
	return err
}

// lookupPages returns the current heap size of the table in 8 KiB blocks, derived
// from pg_relation_size so it reflects live size even for a freshly loaded table
// that has never been ANALYZEd (pg_class.relpages would still read 0). The count
// bounds every shard at the real end of the heap.
func (s *Source) lookupPages(ctx context.Context, qualified string) (int, error) {
	const q = `SELECT (pg_relation_size($1::regclass) / current_setting('block_size')::bigint)::int`
	var pages int
	if err := s.pool.QueryRow(ctx, q, qualified).Scan(&pages); err != nil {
		return 0, err
	}
	return pages, nil
}

// windowBlocks sizes the per-query block window so a window holds roughly page_size
// rows: it samples the live row count of heap block 0 and divides. Bounding the read
// by blocks (not a row LIMIT + keyset) keeps each query a tightly-bounded TID range
// scan, the cost the open-ended cursor used to blow up.
func (s *Source) windowBlocks(ctx context.Context, qualified string) (int, error) {
	// count(*) over block 0 only — a single-block TID range scan.
	q := "SELECT count(*) FROM " + qualified + " t WHERE t.ctid < '(1,0)'::tid"
	var rowsPerBlock int
	if err := s.pool.QueryRow(ctx, q).Scan(&rowsPerBlock); err != nil {
		return 0, err
	}
	if rowsPerBlock < 1 {
		rowsPerBlock = defaultRowsPerBlock
	}
	w := max(s.pageSize/rowsPerBlock, 1)
	return min(w, maxWindowBlocks), nil
}
