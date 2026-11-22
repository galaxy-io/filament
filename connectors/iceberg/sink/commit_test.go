package iceberg

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
)

type manyCommit struct {
	err       error
	resources []string
}

// runManyResources writes 2,000 rows to each of n tables in one run and
// commits them. failCommit names a table whose catalog commit fails.
func runManyResources(t *testing.T, env *testEnv, ingestion filament.IngestionType, n int, failCommit string) manyCommit {
	t.Helper()
	ctx := context.Background()
	var resources []string
	policies := map[string]filament.WritePolicy{}
	for i := range n {
		resource := fmt.Sprintf("t%02d", i)
		resources = append(resources, resource)
		policy := filament.WritePolicyForIngestion(ingestion)
		policy.Resource = resource
		policies[resource] = policy
	}
	sink := New()
	if err := sink.Open(ctx, filament.RunSpec{
		Run: "many", StartedAt: time.Now(), Resources: resources,
		Sink:          filament.Ref{Connector: "iceberg", Config: env.cfg},
		WritePolicies: policies,
	}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sink.Close(ctx) }()
	if failCommit != "" {
		sink.cat = &failingCatalog{Catalog: sink.cat, table: failCommit}
	}
	for _, resource := range resources {
		if err := sink.EnsureSchema(ctx, resource, env.schema); err != nil {
			t.Fatal(err)
		}
	}
	if err := sink.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	for _, resource := range resources {
		batch := idBatch(t, 0, 2000)
		batch.Resource = resource
		if _, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policies[resource]}); err != nil {
			t.Fatal(err)
		}
		batch.Release()
	}
	err := sink.Commit(ctx)
	if err != nil {
		if abortErr := sink.Abort(ctx); abortErr != nil {
			t.Fatal(abortErr)
		}
	}
	return manyCommit{err: err, resources: resources}
}

func tableFiles(t *testing.T, env *testEnv, resource string) int {
	t.Helper()
	files := 0
	dir := filepath.Join(env.dir, "warehouse", "test.db", resource)
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".parquet") {
			files++
		}
		return nil
	})
	return files
}

// TestParallelCommitLandsEveryTable commits 20 tables at the default
// concurrency and checks each holds exactly its rows.
func TestParallelCommitLandsEveryTable(t *testing.T) {
	for _, ingestion := range []filament.IngestionType{filament.IngestionFullAppend, filament.IngestionFullReplace} {
		t.Run(string(ingestion), func(t *testing.T) {
			env := newTestEnv(t)
			commit := runManyResources(t, env, ingestion, 20, "")
			if commit.err != nil {
				t.Fatal(commit.err)
			}
			for _, resource := range commit.resources {
				if rows := tableRows(t, env, resource); rows != 2000 {
					t.Fatalf("%s has %d rows, want 2000", resource, rows)
				}
			}
		})
	}
}

// TestParallelCommitFailureStopsNewCommits fails one table's catalog commit
// with one commit at a time. The run must fail naming that table; every other
// table is either fully committed or untouched, and an untouched table keeps
// no data files after Abort.
func TestParallelCommitFailureStopsNewCommits(t *testing.T) {
	for _, workers := range []int{1, defaultCommitConcurrency} {
		t.Run(fmt.Sprintf("commit_concurrency=%d", workers), func(t *testing.T) {
			env := newTestEnv(t)
			env.cfg["commit_concurrency"] = workers
			commit := runManyResources(t, env, filament.IngestionFullAppend, 20, "t07")
			if commit.err == nil || !strings.Contains(commit.err.Error(), "t07") {
				t.Fatalf("Commit error = %v, want the failure of t07", commit.err)
			}
			committed, untouched := 0, 0
			for _, resource := range commit.resources {
				switch rows := tableRows(t, env, resource); {
				case rows == 2000:
					committed++
				case rows == 0 && resource == "t07":
				case rows == 0:
					untouched++
					if files := tableFiles(t, env, resource); files != 0 {
						t.Fatalf("%s never committed but kept %d data files", resource, files)
					}
				default:
					t.Fatalf("%s has %d rows, want 0 or 2000", resource, rows)
				}
			}
			if workers == 1 && untouched == 0 {
				t.Fatal("one commit at a time kept committing after the failure")
			}
			t.Logf("committed %d, untouched %d", committed, untouched)
		})
	}
}
