package postgres

// Partitioned tables. A partitioned parent is one resource: its rows live in
// its leaf partitions, so a snapshot reads every leaf and emits the rows under
// the parent's name, and the publication sets publish_via_partition_root so
// the WAL names the parent too. A partition selection narrows the snapshot to
// some leaves while the change stream still carries every partition.

import (
	"context"
	"fmt"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
)

var _ filament.PartitionSelector = (*Source)(nil)

// leaf is one leaf partition. name is how a selection names it: the bare
// relation name in the source schema, schema-qualified elsewhere.
type leaf struct {
	name      string
	qualified string
	foreign   bool
}

// Partitions lists a partitioned table's leaf partitions, empty for a plain
// table.
func (s *Source) Partitions(ctx context.Context, resource string) ([]string, error) {
	leaves, _, err := s.leaves(ctx, resource)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(leaves))
	for i, l := range leaves {
		names[i] = l.name
	}
	return names, nil
}

// SelectPartitions narrows each partitioned table's snapshot to the listed
// leaf partitions. An empty list snapshots none of them.
func (s *Source) SelectPartitions(partitions map[string][]string) error {
	selected := make(map[string][]string, len(partitions))
	for table, names := range partitions {
		selected[table] = append([]string{}, names...)
	}
	s.partitions = selected
	return nil
}

// leaves lists table's leaf partitions in name order, including those of
// sub-partitioned children. partitioned is false for a plain table, which has
// none.
func (s *Source) leaves(ctx context.Context, table string) (leaves []leaf, partitioned bool, err error) {
	const q = `
SELECT n.nspname, c.relname, c.relkind = 'f'
FROM pg_partition_tree($1::regclass) t
JOIN pg_class c ON c.oid = t.relid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE t.isleaf
ORDER BY n.nspname, c.relname`
	qualified := pgx.Identifier{s.schema, table}.Sanitize()
	if err := s.pool.QueryRow(ctx, `SELECT relkind = 'p' FROM pg_class WHERE oid = $1::regclass`, qualified).Scan(&partitioned); err != nil {
		return nil, false, fmt.Errorf("postgres source: relation kind of %q: %w", table, err)
	}
	if !partitioned {
		return nil, false, nil
	}
	rows, err := s.pool.Query(ctx, q, qualified)
	if err != nil {
		return nil, false, fmt.Errorf("postgres source: partitions of %q: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var schema, name string
		var foreign bool
		if err := rows.Scan(&schema, &name, &foreign); err != nil {
			return nil, false, err
		}
		l := leaf{name: name, qualified: pgx.Identifier{schema, name}.Sanitize(), foreign: foreign}
		if schema != s.schema {
			l.name = schema + "." + name
		}
		if slices.ContainsFunc(leaves, func(other leaf) bool { return other.name == l.name }) {
			return nil, false, fmt.Errorf("postgres source: %q has two partitions named %q", table, l.name)
		}
		leaves = append(leaves, l)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("postgres source: partitions of %q: %w", table, err)
	}
	return leaves, true, nil
}

// snapshotRelations lists the relations a snapshot of table reads: the table
// itself, or its selected leaf partitions when it is partitioned. A foreign
// leaf has no heap to read by ctid, so reading one fails.
func (s *Source) snapshotRelations(ctx context.Context, table string) ([]string, error) {
	leaves, partitioned, err := s.leaves(ctx, table)
	if err != nil {
		return nil, err
	}
	if !partitioned {
		if _, narrowed := s.partitions[table]; narrowed {
			return nil, fmt.Errorf("postgres source: %q is not partitioned, so its partition selection cannot apply", table)
		}
		return []string{pgx.Identifier{s.schema, table}.Sanitize()}, nil
	}
	selected, narrowed := s.partitions[table]
	for _, name := range selected {
		if !slices.ContainsFunc(leaves, func(l leaf) bool { return l.name == name }) {
			return nil, fmt.Errorf("postgres source: %q has no partition %q", table, name)
		}
	}
	var relations []string
	for _, l := range leaves {
		if narrowed && !slices.Contains(selected, l.name) {
			continue
		}
		if l.foreign {
			return nil, fmt.Errorf("postgres source: partition %q of %q is a foreign table", l.name, table)
		}
		relations = append(relations, l.qualified)
	}
	return relations, nil
}
