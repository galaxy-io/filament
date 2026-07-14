// Package postgres implements resumable full-snapshot reads from a PostgreSQL source.
//
// At plan time (before the first row is read) the router asks one question of
// Postgres and combines the answer with a few facts from the run config:
//
//	pg_stats.correlation for the leading PK column
//	  ↓ (between -1 and 1; 1 = rows are physically in key order; 0 = random)
//
//	Is this a CDC run with a logical slot available?  → ModeSlot     (not yet existing)
//	Is the table declared append-only?                → ModeCtidAppendOnly (no xmin, just blocks)
//	|correlation| >= 0.8?                             → ModeKeyset   (index scan already near-sequential)
//	PK is mutable (rows can change their key)?        → ModeCtidXmin (physical blocks + repair pass)
//	Otherwise (random keys, immutable PK)             → ModeBitmap   (bitmap heap scan, same correctness as keyset)
//
// The chosen mode is frozen into the checkpoint at plan time and reused on every resume
// of the same run, so the shard layout never changes mid-flight.
//
// # What each mode actually does
//
//   - ModeKeyset — "follow the index in order"
//     Issues ORDER BY pk LIMIT page queries. The PK index is walked in key order, one
//     page at a time, and the last key seen becomes the cursor for the next page.
//     Works great when the heap is already in the same order as the key (high correlation),
//     because each page is one contiguous disk read. Resumable to the exact row.
//
//   - ModeBitmap — "collect all the addresses first, then fetch blocks in order"
//     Splits the key space into sub-ranges (same boundaries as keyset). Each sub-range is
//     read with a plain range predicate and NO ORDER BY, so Postgres can collect all matching
//     index TIDs, sort them by heap block, and read those blocks in ascending order with
//     prefetch. This beats keyset for randomly-keyed tables because it avoids the random I/O
//     pattern that ORDER BY forces. Resume is all-or-nothing per sub-range: when a sub-range
//     drains fully the reader sends a Drained sentinel; the pipeline counts written rows and
//     marks the shard Done. Interrupted sub-ranges are re-read whole on resume.
//
//   - ModeCtidXmin — "read the heap directly, then patch up anything that moved"
//     Divides the table into block-range shards (ctid = physical heap address). Reads blocks
//     in order — fast sequential I/O, no index needed. The hard part is safety across a resume
//     boundary, handled by three guards:
//     (1) Rewrite guard: the filenode at run-start is stamped; if VACUUM FULL / CLUSTER /
//     TRUNCATE rewrites the heap between runs, the cursor is stale → re-read from scratch.
//     (2) Horizon-compare reconciliation: the run-start xmin horizon H1 is stamped; before
//     the run closes, completed block ranges are re-scanned for rows with age(xmin) <= age(H1)
//     (i.e. rows that were written or moved after run-start). The idempotent sink absorbs
//     any over-delivery.
//     (3) Freeze guard: if relfrozenxid advanced past H1 (vacuum froze xids), the age()
//     comparison is unreliable → unfiltered rescan of completed ranges.
//
//   - ModeCtidAppendOnly — like ModeCtidXmin but without the xmin apparatus; rows never
//     move or update, so the horizon-compare pass is skipped and resume just continues
//     past the recorded tail. (Not yet wired into the router; declared append-only sources
//     route here explicitly.)
//
//   - ModeSlot — a logical replication slot pins an LSN so updated/moved rows return via
//     WAL replay. Correct for as-of-completion semantics and CDC. (Arrives with CDC; the
//     router will prefer this when slot + CDC are both available.)
//
// # Decision table
//
//	Condition                                         Mode
//	─────────────────────────────────────────────────────────────────
//	CDC run + logical slot available                  ModeSlot
//	AppendOnly declared                               ModeCtidAppendOnly
//	|corr| >= 0.8  (heap ≈ key order)                 ModeKeyset
//	PK mutable                                        ModeCtidXmin
//	Default (random keys, immutable PK)               ModeBitmap
//
// # Routing code path
//
//	PlanResume (keyset.go)
//	  └─ resolveMode (bitmap.go)            ← reads s.readMode config + auto probe
//	       └─ Route (readmode.go)           ← queries pg_stats.correlation
//	            └─ chooseMode (readmode.go) ← pure decision function, unit-tested
package postgres
