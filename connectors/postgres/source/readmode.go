package postgres

// Read-mode router. A resumable run
// picks how to read a table from one cheap plan-time probe — pg_stats.correlation for the
// leading key column, the statistical correlation between physical row order and the key's
// order — combined with signals the probe cannot derive (PK mutability, an append-only
// declaration, slot/CDC availability, PG version).
//
// The axis that actually matters is PK mutability, not key type: a key-space cursor
// (keyset, bitmap) is correct for any immutable-PK source and only loses PK-mutated rows;
// the physical tier (ctid+xmin, slot) earns its cost solely for PK-mutable or
// as-of-completion semantics. So a randomly-keyed table with an immutable PK routes to
// bitmap — a *faster* keyset at the *same* correctness tier, not a riskier one.

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
)

// ReadMode is the chosen extraction strategy for a resumable resource.
type ReadMode int

const (
	// ModeKeyset reads in key order via the PK index — already sequential when the leading
	// key correlates with heap order, and resumable from a compact key cursor.
	ModeKeyset ReadMode = iota
	// ModeBitmap reads each key sub-range with no ORDER BY so the planner is free to pick a
	// bitmap heap scan (ascending block order + prefetch). Default for random immutable keys.
	ModeBitmap
	// ModeCtidAppendOnly reads physical block ranges on a declared append-only table — no
	// xmin apparatus, since rows never move; resume continues past the recorded tail.
	ModeCtidAppendOnly
	// ModeCtidXmin is the general physical tier: block-range cursor + horizon-compare repair.
	// Catches arbitrary churn including PK moves.
	ModeCtidXmin
	// ModeSlot is the premium physical tier: a logical replication slot pins an LSN and moved
	// rows return via WAL replay.
	ModeSlot
)

func (m ReadMode) String() string {
	switch m {
	case ModeKeyset:
		return "keyset"
	case ModeBitmap:
		return "bitmap"
	case ModeCtidAppendOnly:
		return "ctid_append_only"
	case ModeCtidXmin:
		return "ctid_xmin"
	case ModeSlot:
		return "slot"
	default:
		return "unknown"
	}
}

// correlationThreshold: at or above this |correlation| the heap is ordered closely enough
// to the leading key that the index-ordered keyset read is already near-sequential, so
// there is nothing for bitmap to recover. Below it, ordered delivery thrashes the heap and
// bitmap's block-ordered + prefetched scan wins. 0.8 is deliberately conservative — a false
// "keyset" only forfeits a speedup, never correctness.
const correlationThreshold = 0.8

// RouteSignals are the inputs the correlation probe cannot derive from catalog stats: they
// come from the run config, the source declaration, or the environment.
type RouteSignals struct {
	AppendOnly    bool // source declared the table append-only (rows never move/update)
	PKMutable     bool // the primary key can change → needs the physical tier for completeness
	SlotAvailable bool // a logical replication slot is usable (wal_level=logical + privilege)
	CDC           bool // this run feeds change-data-capture (slot pins the snapshot boundary)
}

// chooseMode is the pure routing decision, isolated from any I/O so it is exhaustively
// unit-testable. corr is the leading key's pg_stats.correlation in [-1, 1]; haveCorr is
// false when stats are unavailable even after ANALYZE (treated as low correlation —
// random — which is the safe assumption for an unknown distribution).
func chooseMode(corr float64, haveCorr bool, sig RouteSignals) ReadMode {
	switch {
	case sig.SlotAvailable && sig.CDC:
		return ModeSlot
	case sig.AppendOnly:
		return ModeCtidAppendOnly
	case haveCorr && math.Abs(corr) >= correlationThreshold:
		return ModeKeyset // physical order already tracks key order
	case sig.PKMutable:
		return ModeCtidXmin // random heap AND mutable PK → physical cursor + repair
	default:
		return ModeBitmap // random heap, immutable PK → faster keyset, same correctness tier
	}
}

// Route runs the correlation probe for a table's leading key and returns the chosen mode
// along with the observed correlation (for an observability fact). It never fails the run:
// a probe error degrades to "no correlation" (random) and routes on the signals alone.
func (s *Source) Route(ctx context.Context, table string, sig RouteSignals) (ReadMode, float64, bool) {
	pks, err := s.lookupPrimaryKey(ctx, s.schema, table)
	if err != nil || len(pks) == 0 {
		// No key → not resumable by key space anyway; bitmap/keyset don't apply.
		return chooseMode(0, false, sig), 0, false
	}
	corr, ok := s.leadingKeyCorrelation(ctx, table, pks[0].name)
	return chooseMode(corr, ok, sig), corr, ok
}

// leadingKeyCorrelation reads pg_stats.correlation for the leading key column, running a
// one-shot ANALYZE when stats are absent (a just-loaded table has none). correlation is
// null for some types/distributions; that and any error report ok=false (caller treats it
// as random).
func (s *Source) leadingKeyCorrelation(ctx context.Context, table, col string) (float64, bool) {
	corr, ok, err := s.readCorrelation(ctx, table, col)
	if err == nil && ok {
		return corr, true
	}
	// Stats missing → ANALYZE just this table (seconds, sampled) and read once more.
	qualified := pgx.Identifier{s.schema, table}.Sanitize()
	if _, err := s.pool.Exec(ctx, "ANALYZE "+qualified, pgx.QueryExecModeSimpleProtocol); err != nil {
		return 0, false
	}
	corr, ok, err = s.readCorrelation(ctx, table, col)
	if err != nil {
		return 0, false
	}
	return corr, ok
}

// readCorrelation fetches pg_stats.correlation; ok=false when there is no stats row or the
// value is null.
func (s *Source) readCorrelation(ctx context.Context, table, col string) (float64, bool, error) {
	const q = `SELECT correlation FROM pg_stats WHERE schemaname=$1 AND tablename=$2 AND attname=$3`
	var corr *float64
	if err := s.pool.QueryRow(ctx, q, s.schema, table, col).Scan(&corr); err != nil {
		if err == pgx.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("read correlation %q.%q: %w", table, col, err)
	}
	if corr == nil {
		return 0, false, nil
	}
	return *corr, true, nil
}
