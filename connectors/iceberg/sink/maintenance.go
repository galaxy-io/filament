package iceberg

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	iceberg "github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/table"
	"github.com/apache/iceberg-go/table/compaction"
)

// Maintenance keeps a table cheap to read and commit to under frequent small
// snapshots. It compacts small data files, merges manifests, and expires old
// snapshots. Whether it is due is read from the table itself, because the
// runner opens a fresh sink for every continuous epoch.
const (
	maintenanceCommits  = 10
	maintenanceInterval = 10 * time.Minute
	maintainedAtKey     = "filament.maintenance.last-run-ms"

	// Retention honors the table properties Spark and Trino users already set.
	maxSnapshotAgeKey         = "history.expire.max-snapshot-age-ms"
	maxSnapshotAgeDefault     = 5 * 24 * time.Hour
	minSnapshotsToKeepKey     = "history.expire.min-snapshots-to-keep"
	minSnapshotsToKeepDefault = 1
)

// maintainCommitted runs due maintenance on each table committed by the stage.
// Failures are logged and never fail the commit, which is already durable.
func (s *Sink) maintainCommitted(ctx context.Context, st *stage) {
	if !s.maintenance {
		return
	}
	for resource := range st.committed {
		s.mu.Lock()
		it := s.tables[resource]
		s.mu.Unlock()
		if it == nil || !maintenanceDue(it.tbl, time.Now()) {
			continue
		}
		if err := maintain(ctx, it); err != nil {
			slog.WarnContext(ctx, "Iceberg table maintenance failed", "component", "iceberg", "event.name", "iceberg.maintenance.failed", "run_id", s.run, "resource", resource, "error", err)
			continue
		}
		slog.InfoContext(ctx, "Iceberg table maintenance completed", "component", "iceberg", "event.name", "iceberg.maintenance.completed", "run_id", s.run, "resource", resource)
	}
}

// maintenanceDue reports whether enough commits or time have passed since the
// last maintenance run stamped on the table.
func maintenanceDue(tbl *table.Table, now time.Time) bool {
	cur := tbl.CurrentSnapshot()
	if cur == nil {
		return false
	}
	last := tbl.Properties().GetInt64(maintainedAtKey, 0)
	commits := 0
	for _, snap := range table.AncestorsOf(cur.SnapshotID, tbl.SnapshotByID) {
		if snap.TimestampMs <= last {
			break
		}
		commits++
	}
	if commits == 0 {
		return false
	}
	return commits >= maintenanceCommits || now.Sub(time.UnixMilli(last)) >= maintenanceInterval
}

// maintain compacts, then merges manifests, then expires snapshots, each as its
// own commit. The table handle follows every step that lands.
func maintain(ctx context.Context, it *iceTable) error {
	steps := []struct {
		name string
		run  func(context.Context, *table.Table) (*table.Table, error)
	}{
		{"compact data files", compactDataFiles},
		{"rewrite manifests", rewriteManifests},
		{"expire snapshots", expireSnapshots},
	}
	for _, step := range steps {
		tbl, err := step.run(ctx, it.tbl)
		if err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
		it.tbl = tbl
		it.schema = tbl.Schema()
	}
	return nil
}

// compactDataFiles bin-packs small data files toward the table's target file
// size and drops equality deletes the rewrite leaves without a target.
func compactDataFiles(ctx context.Context, tbl *table.Table) (*table.Table, error) {
	cfg := compaction.DefaultConfig()
	if target := tbl.Properties().GetInt64(table.WriteTargetFileSizeBytesKey, 0); target > 0 {
		cfg.TargetFileSizeBytes = target
		cfg.MinFileSizeBytes = target * 3 / 4
		cfg.MaxFileSizeBytes = target * 9 / 5
	}
	plan, err := compaction.Analyze(ctx, tbl, cfg)
	if err != nil || len(plan.Groups) == 0 {
		return tbl, err
	}

	groups := make([]table.CompactionTaskGroup, len(plan.Groups))
	rewritten := map[string]struct{}{}
	for i, g := range plan.Groups {
		groups[i] = table.CompactionTaskGroup{PartitionKey: g.PartitionKey, Tasks: g.Tasks, TotalSizeBytes: g.TotalSizeBytes}
		for _, task := range g.Tasks {
			rewritten[task.File.FilePath()] = struct{}{}
		}
	}
	fs, err := tbl.FS(ctx)
	if err != nil {
		return tbl, err
	}
	dead, err := compaction.CollectDeadEqualityDeletes(ctx, fs, tbl.CurrentSnapshot(), rewritten)
	if err != nil {
		return tbl, fmt.Errorf("collect dead equality deletes: %w", err)
	}

	txn := tbl.NewTransaction()
	if _, err := txn.RewriteDataFiles(ctx, groups, table.RewriteDataFilesOptions{ExtraDeleteFilesToRemove: dead}); err != nil {
		return tbl, err
	}
	return txn.Commit(ctx)
}

func rewriteManifests(ctx context.Context, tbl *table.Table) (*table.Table, error) {
	txn := tbl.NewTransaction()
	res, err := txn.RewriteManifests(ctx)
	if err != nil || res.IsNoOp() {
		return tbl, err
	}
	return txn.Commit(ctx)
}

// expireSnapshots also stamps the run on the table, so the next sink finds
// maintenance not due even when nothing was rewritten.
func expireSnapshots(ctx context.Context, tbl *table.Table) (*table.Table, error) {
	props := tbl.Properties()
	maxAge := time.Duration(props.GetInt64(maxSnapshotAgeKey, maxSnapshotAgeDefault.Milliseconds())) * time.Millisecond
	txn := tbl.NewTransaction()
	if err := txn.ExpireSnapshots(
		table.WithOlderThan(maxAge),
		table.WithRetainLast(props.GetInt(minSnapshotsToKeepKey, minSnapshotsToKeepDefault)),
	); err != nil {
		return tbl, err
	}
	if err := txn.SetProperties(iceberg.Properties{maintainedAtKey: strconv.FormatInt(time.Now().UnixMilli(), 10)}); err != nil {
		return tbl, err
	}
	return txn.Commit(ctx)
}
