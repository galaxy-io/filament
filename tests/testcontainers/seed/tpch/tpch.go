// Package tpch registers TPC-H seed scenarios in the global registry.
// Import it blank to activate:
//
//	import _ "github.com/galaxy-io/filament/tests/testcontainers/seed/tpch"
//
// Seeding uses the DuckDB CLI (must be on PATH: `brew install duckdb`).
// DuckDB generates TPC-H data and exports each table to a CSV temp file;
// this package then loads the CSVs into Postgres via pgx COPY FROM STDIN,
// giving per-table progress output and avoiding the DuckDB postgres extension.
package tpch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament/tests/testcontainers/seed"
)

// Tables are the eight standard TPC-H tables, ordered small→large by row count
// at any scale factor. lineitem dominates (~6M rows/SF).
var Tables = []string{
	"region", "nation", "supplier", "customer",
	"part", "partsupp", "orders", "lineitem",
}

// tableDDL is the CREATE TABLE statement for each TPC-H table in Postgres.
var tableDDL = map[string]string{
	"region":   `CREATE TABLE region (r_regionkey integer, r_name text, r_comment text)`,
	"nation":   `CREATE TABLE nation (n_nationkey integer, n_name text, n_regionkey integer, n_comment text)`,
	"supplier": `CREATE TABLE supplier (s_suppkey integer, s_name text, s_address text, s_nationkey integer, s_phone text, s_acctbal numeric, s_comment text)`,
	"customer": `CREATE TABLE customer (c_custkey integer, c_name text, c_address text, c_nationkey integer, c_phone text, c_acctbal numeric, c_mktsegment text, c_comment text)`,
	"part":     `CREATE TABLE part (p_partkey integer, p_name text, p_mfgr text, p_brand text, p_type text, p_size integer, p_container text, p_retailprice numeric, p_comment text)`,
	"partsupp": `CREATE TABLE partsupp (ps_partkey integer, ps_suppkey integer, ps_availqty integer, ps_supplycost numeric, ps_comment text)`,
	"orders":   `CREATE TABLE orders (o_orderkey integer, o_custkey integer, o_orderstatus text, o_totalprice numeric, o_orderdate date, o_orderpriority text, o_clerk text, o_shippriority integer, o_comment text)`,
	"lineitem": `CREATE TABLE lineitem (l_orderkey integer, l_partkey integer, l_suppkey integer, l_linenumber integer, l_quantity numeric, l_extendedprice numeric, l_discount numeric, l_tax numeric, l_returnflag text, l_linestatus text, l_shipdate date, l_commitdate date, l_receiptdate date, l_shipinstruct text, l_shipmode text, l_comment text)`,
}

// primaryKeys maps each TPC-H table to its PRIMARY KEY columns for ALTER TABLE.
var primaryKeys = map[string]string{
	"region":   "ALTER TABLE region   ADD PRIMARY KEY (r_regionkey)",
	"nation":   "ALTER TABLE nation   ADD PRIMARY KEY (n_nationkey)",
	"supplier": "ALTER TABLE supplier ADD PRIMARY KEY (s_suppkey)",
	"customer": "ALTER TABLE customer ADD PRIMARY KEY (c_custkey)",
	"part":     "ALTER TABLE part     ADD PRIMARY KEY (p_partkey)",
	"partsupp": "ALTER TABLE partsupp ADD PRIMARY KEY (ps_partkey, ps_suppkey)",
	"orders":   "ALTER TABLE orders   ADD PRIMARY KEY (o_orderkey)",
	"lineitem": "ALTER TABLE lineitem ADD PRIMARY KEY (l_orderkey, l_linenumber)",
}

// scaleRows gives the approximate per-table row count for the three pre-seeded
// SFs. Used by verifier to check counts; dynamic SFs use live query counts.
var scaleRows = map[float64]map[string]int64{
	0.1: {"region": 5, "nation": 25, "supplier": 1_000, "customer": 15_000, "part": 20_000, "partsupp": 80_000, "orders": 150_000, "lineitem": 600_572},
	1:   {"region": 5, "nation": 25, "supplier": 10_000, "customer": 150_000, "part": 200_000, "partsupp": 800_000, "orders": 1_500_000, "lineitem": 6_001_215},
	5:   {"region": 5, "nation": 25, "supplier": 50_000, "customer": 750_000, "part": 1_000_000, "partsupp": 4_000_000, "orders": 7_500_000, "lineitem": 29_999_795},
}

func init() {
	for _, sf := range []float64{0.1, 1, 5} {
		RegisterSF(sf)
	}
}

// RegisterSF registers a TPC-H scenario for the given scale factor. The three
// standard SFs (0.1, 1, 5) are registered automatically; call this to add
// others (e.g. RegisterSF(0.01) for fast smoke tests or RegisterSF(10) for
// large load runs).
func RegisterSF(sf float64) {
	approxRows := totalRows(sf)
	seed.Register(seed.Scenario{
		Name:        fmt.Sprintf("tpch-sf%.4g", sf),
		Description: fmt.Sprintf("TPC-H scale factor %.4g — 8 tables, ~%s rows total", sf, humanRows(approxRows)),
		Spec: seed.Spec{
			Name:         fmt.Sprintf("tpch-sf%.4g", sf),
			Tables:       len(Tables),
			RowsPerTable: int(approxRows) / len(Tables),
		},
		Seeders: map[string]seed.SeederFunc{
			"postgres":   seeder(sf),
			"postgresql": seeder(sf),
		},
		Droppers: map[string]seed.DropFunc{
			"postgres":   dropAll,
			"postgresql": dropAll,
		},
		Verifiers: map[string]seed.VerifyFunc{
			"postgres":   verifier(sf),
			"postgresql": verifier(sf),
		},
	})
}

func seeder(sf float64) seed.SeederFunc {
	return func(ctx context.Context, dsn string) (seed.Manifest, error) {
		if _, err := exec.LookPath("duckdb"); err != nil {
			return seed.Manifest{}, fmt.Errorf("duckdb not found on PATH — install with: brew install duckdb")
		}

		tmpDir, err := os.MkdirTemp("", "tpch-")
		if err != nil {
			return seed.Manifest{}, fmt.Errorf("mktemp: %w", err)
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		seed.Progressf(ctx, "  generating TPC-H SF=%.4g via DuckDB…\n", sf)
		if err := generateCSVs(ctx, sf, tmpDir); err != nil {
			return seed.Manifest{}, err
		}

		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return seed.Manifest{}, fmt.Errorf("connect: %w", err)
		}
		defer pool.Close()

		m := seed.Manifest{
			Spec: seed.Spec{Name: fmt.Sprintf("tpch-sf%.4g", sf), Tables: len(Tables)},
		}
		for _, tbl := range Tables {
			rows, err := loadCSV(ctx, pool, tbl, tmpDir)
			if err != nil {
				return seed.Manifest{}, err
			}
			m.Tables = append(m.Tables, seed.Table{Name: tbl, Rows: rows})
			seed.Progressf(ctx, "  %-12s  %d rows\n", tbl, rows)
		}

		if err := applyPrimaryKeys(ctx, pool); err != nil {
			return seed.Manifest{}, err
		}

		return m, nil
	}
}

// dropAll drops all TPC-H tables regardless of scale factor.
var dropAll seed.DropFunc = func(ctx context.Context, dsn string) error {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()
	for _, tbl := range Tables {
		// Table names are package constants — safe to interpolate.
		if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS "+tbl); err != nil {
			return fmt.Errorf("drop %s: %w", tbl, err)
		}
		seed.Progressf(ctx, "  dropped %s\n", tbl)
	}
	return nil
}

func verifier(sf float64) seed.VerifyFunc {
	return func(ctx context.Context, dsn string) (seed.Manifest, error) {
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			return seed.Manifest{}, fmt.Errorf("connect: %w", err)
		}
		defer pool.Close()

		expected := scaleRows[sf] // nil for dynamic SFs — skip count check
		m := seed.Manifest{
			Spec: seed.Spec{Name: fmt.Sprintf("tpch-sf%.4g", sf), Tables: len(Tables)},
		}
		for _, tbl := range Tables {
			var count int64
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+tbl).Scan(&count); err != nil {
				return seed.Manifest{}, fmt.Errorf("count %s: %w", tbl, err)
			}
			t := seed.Table{Name: tbl, Rows: count}
			if expected != nil {
				if exp, ok := expected[tbl]; ok && count != exp {
					t.Error = fmt.Sprintf("MISMATCH: got %d, expected %d", count, exp)
				}
			}
			m.Tables = append(m.Tables, t)
			seed.Progressf(ctx, "  %-12s  %d rows\n", tbl, count)
		}
		return m, nil
	}
}

// generateCSVs runs DuckDB to generate all TPC-H tables as CSVs in dir.
func generateCSVs(ctx context.Context, sf float64, dir string) error {
	copies := make([]string, len(Tables))
	for i, t := range Tables {
		copies[i] = fmt.Sprintf("COPY %s TO '%s' (HEADER FALSE);", t, filepath.Join(dir, t+".csv"))
	}

	sql := fmt.Sprintf("INSTALL tpch; LOAD tpch; CALL dbgen(sf=%g);\n%s",
		sf, strings.Join(copies, "\n"))

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "duckdb", ":memory:", "-c", sql)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("duckdb: %w\n%s", err, stderr.String())
	}
	return nil
}

// loadCSV bulk-loads a CSV file into the named Postgres table using COPY FROM.
// Returns the number of rows loaded as reported by Postgres.
func loadCSV(ctx context.Context, pool *pgxpool.Pool, tbl, dir string) (int64, error) {
	ddl := tableDDL[tbl]
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS "+tbl+"; "+ddl); err != nil {
		return 0, fmt.Errorf("create %s: %w", tbl, err)
	}

	f, err := os.Open(filepath.Join(dir, tbl+".csv"))
	if err != nil {
		return 0, fmt.Errorf("open csv %s: %w", tbl, err)
	}
	defer func() { _ = f.Close() }()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return 0, fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	ct, err := conn.Conn().PgConn().CopyFrom(ctx, f, "COPY "+tbl+" FROM STDIN CSV")
	if err != nil {
		return 0, fmt.Errorf("copy %s: %w", tbl, err)
	}
	return ct.RowsAffected(), nil
}

func applyPrimaryKeys(ctx context.Context, pool *pgxpool.Pool) error {
	for tbl, ddl := range primaryKeys {
		if _, err := pool.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("primary key %s: %w", tbl, err)
		}
	}
	return nil
}

func totalRows(sf float64) int64 {
	rows, ok := scaleRows[sf]
	if !ok {
		return 0
	}
	var total int64
	for _, n := range rows {
		total += n
	}
	return total
}

func humanRows(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
