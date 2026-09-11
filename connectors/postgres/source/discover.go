package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
)

// Discover lists tables in the configured schema with their primary keys and row estimates.
func (s *Source) Discover(ctx context.Context, _ filament.DiscoverOpts) (filament.DiscoverResult, error) {
	if s.pool == nil {
		return filament.DiscoverResult{}, fmt.Errorf("postgres source: discover before configure")
	}
	const q = `
SELECT
	c.relname,
	COALESCE(string_agg(a.attname, ',' ORDER BY k.ord), '') AS primary_key,
	GREATEST(c.reltuples::bigint, 0) AS estimated_rows
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_index i ON i.indrelid = c.oid AND i.indisprimary
LEFT JOIN LATERAL unnest(i.indkey) WITH ORDINALITY AS k(attnum, ord) ON true
LEFT JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = k.attnum
WHERE n.nspname = $1
  AND c.relkind IN ('r', 'p')
GROUP BY c.relname, c.reltuples
ORDER BY c.relname`
	rows, err := s.pool.Query(ctx, q, s.schema)
	if err != nil {
		return filament.DiscoverResult{}, fmt.Errorf("postgres source: discover tables: %w", err)
	}
	defer rows.Close()

	var resources []filament.Resource
	for rows.Next() {
		var name, pkCSV string
		var estimated int64
		if err := rows.Scan(&name, &pkCSV, &estimated); err != nil {
			return filament.DiscoverResult{}, fmt.Errorf("postgres source: scan table: %w", err)
		}
		var pk []string
		if pkCSV != "" {
			pk = strings.Split(pkCSV, ",")
		}
		resources = append(resources, filament.Resource{
			Name:       name,
			Selectable: true,
			PrimaryKey: pk,
			Estimated:  estimated,
		})
	}
	if err := rows.Err(); err != nil {
		return filament.DiscoverResult{}, fmt.Errorf("postgres source: discover rows: %w", err)
	}
	return filament.DiscoverResult{Resources: resources}, nil
}
