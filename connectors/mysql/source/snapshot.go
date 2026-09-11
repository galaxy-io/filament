package mysql

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

// Extract plans every requested table into keyset shards and pages them out through
// the sink. With Parallelism > 1 the shards are read concurrently (bounded by
// Parallelism), each inside its own consistent-snapshot transaction; paging within a
// shard stays sequential. The first shard error cancels the rest and is returned.
func (s *Source) Extract(ctx context.Context, sink arrowbatch.Inlet, opts filament.ExtractOpts) error {
	var jobs []func(context.Context, querier) error
	for _, table := range opts.Resources {
		tjobs, err := s.tableJobs(ctx, sink, table, nil, opts.Limit)
		if err != nil {
			return err
		}
		jobs = append(jobs, tjobs...)
	}
	return s.runConcurrent(ctx, opts.Parallelism, jobs)
}

// tableJobs plans one table into its shard read jobs: a fresh keyset plan for a keyed
// table, or a single streaming full scan for a keyless one. prev seeds each keyset
// shard's cursor when resuming.
func (s *Source) tableJobs(ctx context.Context, sink arrowbatch.Inlet, table string, ks *keysetPlan, limit int) ([]func(context.Context, querier) error, error) {
	dec, err := s.decoderFor(ctx, table)
	if err != nil {
		return nil, err
	}
	qualified := quoteIdent(s.database) + "." + quoteIdent(table)

	if len(dec.pks) == 0 {
		// Keyless: one streaming scan.
		return []func(context.Context, querier) error{func(ctx context.Context, q querier) error {
			w, err := sink.Builder(table, 0, dec.schema)
			if err != nil {
				return err
			}
			return s.extractKeyless(ctx, w, q, table, qualified, dec, limit)
		}}, nil
	}

	if ks == nil {
		fresh, err := s.planKeyset(ctx, table, qualified, dec.pks)
		if err != nil {
			return nil, err
		}
		ks = &fresh
	}
	shards, err := keyShardsFrom(table, qualified, dec, *ks)
	if err != nil {
		return nil, err
	}
	var jobs []func(context.Context, querier) error
	for _, sh := range shards {
		jobs = append(jobs, func(ctx context.Context, q querier) error {
			w, err := sink.Builder(sh.table, sh.part, sh.dec.schema)
			if err != nil {
				return err
			}
			return s.extractKeysetShard(ctx, w, q, sh, limit)
		})
	}
	return jobs, nil
}

// extractKeyless streams a keyless table in one pass into w. There is no stable key
// to page or resume by, so the whole table is read in a single query.
func (s *Source) extractKeyless(ctx context.Context, w arrowbatch.RowWriter, q querier, table, qualified string, dec *rowDecoder, limit int) error {
	rows, err := q.QueryContext(ctx, "SELECT "+dec.selectList+" FROM "+qualified+" t")
	if err != nil {
		return fmt.Errorf("scan %q: %w", table, err)
	}
	defer func() { _ = rows.Close() }()
	_, _, err = dec.appendRows(rows, w, nil, nil, limit)
	return err
}
