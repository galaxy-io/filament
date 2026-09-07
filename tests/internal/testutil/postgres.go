//go:build integration || e2e

package testutil

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresJSONRows reads selected columns as deterministic JSON rows. Explicit
// columns keep sink-added metadata out of source/destination comparisons.
func PostgresJSONRows(t testing.TB, ctx context.Context, pool *pgxpool.Pool, table, orderBy string, columns []string) []string {
	t.Helper()
	qualified := pgx.Identifier{table}.Sanitize()
	order := pgx.Identifier{orderBy}.Sanitize()
	selectList := make([]string, len(columns))
	for i, column := range columns {
		selectList[i] = pgx.Identifier{column}.Sanitize()
	}
	rows, err := pool.Query(ctx, "SELECT row_to_json(t)::text FROM (SELECT "+strings.Join(selectList, ", ")+" FROM "+qualified+" ORDER BY "+order+") t")
	if err != nil {
		t.Fatalf("read postgres %s: %v", table, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatal(err)
		}
		out = append(out, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
