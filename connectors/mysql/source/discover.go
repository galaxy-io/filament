package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
)

// Discover lists tables in the configured database with their primary keys and row estimates.
func (s *Source) Discover(ctx context.Context, _ filament.DiscoverOpts) (filament.DiscoverResult, error) {
	if s.db == nil {
		return filament.DiscoverResult{}, fmt.Errorf("mysql source: discover before configure")
	}
	const q = `
SELECT
	t.TABLE_NAME,
	COALESCE((
		SELECT GROUP_CONCAT(k.COLUMN_NAME ORDER BY k.ORDINAL_POSITION SEPARATOR ',')
		FROM information_schema.KEY_COLUMN_USAGE k
		WHERE k.TABLE_SCHEMA = t.TABLE_SCHEMA AND k.TABLE_NAME = t.TABLE_NAME
		  AND k.CONSTRAINT_NAME = 'PRIMARY'
	), '') AS primary_key,
	COALESCE(t.TABLE_ROWS, 0) AS estimated_rows
FROM information_schema.TABLES t
WHERE t.TABLE_SCHEMA = ? AND t.TABLE_TYPE = 'BASE TABLE'
ORDER BY t.TABLE_NAME`
	rows, err := s.db.QueryContext(ctx, q, s.database)
	if err != nil {
		return filament.DiscoverResult{}, fmt.Errorf("mysql source: discover tables: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var resources []filament.Resource
	for rows.Next() {
		var name, pkCSV string
		var estimated int64
		if err := rows.Scan(&name, &pkCSV, &estimated); err != nil {
			return filament.DiscoverResult{}, fmt.Errorf("mysql source: scan table: %w", err)
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
		return filament.DiscoverResult{}, fmt.Errorf("mysql source: discover rows: %w", err)
	}
	return filament.DiscoverResult{Resources: resources}, nil
}
