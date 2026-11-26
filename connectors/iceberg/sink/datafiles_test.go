package iceberg

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
)

// rotateEveryBatch makes each append batch its own data file.
func rotateEveryBatch(t *testing.T) {
	previous := streamRotateAfter
	streamRotateAfter = 0
	t.Cleanup(func() { streamRotateAfter = previous })
}

// parquetFiles lists data files under the test warehouse.
func parquetFiles(t *testing.T, env *testEnv) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(filepath.Join(env.dir, "warehouse"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".parquet") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

type openRun struct {
	sink   *Sink
	policy filament.WritePolicy
}

func openAppendRun(t *testing.T, env *testEnv) openRun {
	t.Helper()
	ctx := context.Background()
	policy := eventsPolicy(filament.IngestionFullAppend)
	sink := New()
	if err := sink.Open(ctx, filament.RunSpec{
		Run: "run", StartedAt: time.Now(), Resources: []string{"events"},
		Sink:          filament.Ref{Connector: "iceberg", Config: env.cfg},
		WritePolicies: map[string]filament.WritePolicy{"events": policy},
	}); err != nil {
		t.Fatal(err)
	}
	if err := sink.EnsureSchema(ctx, "events", env.schema); err != nil {
		t.Fatal(err)
	}
	if err := sink.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	return openRun{sink: sink, policy: policy}
}

func (r openRun) apply(t *testing.T, batches int) {
	t.Helper()
	for n := range batches {
		batch := idBatch(t, n*1000, 1000)
		if _, err := r.sink.Apply(context.Background(), batch, filament.ApplyOptions{Policy: r.policy}); err != nil {
			t.Fatal(err)
		}
		batch.Release()
	}
}

// TestAppendWritesFilesDuringRunAndCommitsAll rotates the stream on every
// batch, so the run writes many files before Commit, and checks that none is
// visible until Commit and that Commit attaches every one.
func TestAppendWritesFilesDuringRunAndCommitsAll(t *testing.T) {
	rotateEveryBatch(t)
	env := newTestEnv(t)
	run := openAppendRun(t, env)
	run.apply(t, 6)

	if written := len(parquetFiles(t, env)); written < 5 {
		t.Fatalf("%d data files written before Commit, want the run's files written as it goes", written)
	}
	if rows := tableRows(t, env, "events"); rows != 0 {
		t.Fatalf("%d rows visible before Commit", rows)
	}
	if err := run.sink.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := run.sink.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rows := tableRows(t, env, "events"); rows != 6000 {
		t.Fatalf("table has %d rows, want 6000", rows)
	}
	if files := env.dataFiles(t); files != 6 {
		t.Fatalf("snapshot has %d data files, want 6", files)
	}
}

// TestAppendAbortDeletesWrittenFiles aborts a run that already wrote files and
// checks that none is left in the warehouse and the table is unchanged.
func TestAppendAbortDeletesWrittenFiles(t *testing.T) {
	rotateEveryBatch(t)
	env := newTestEnv(t)
	run := openAppendRun(t, env)
	run.apply(t, 4)
	if written := len(parquetFiles(t, env)); written == 0 {
		t.Fatal("no data files written before Abort")
	}
	if err := run.sink.Abort(context.Background()); err != nil {
		t.Fatal(err)
	}
	if left := parquetFiles(t, env); len(left) != 0 {
		t.Fatalf("Abort left data files: %v", left)
	}
	if rows := tableRows(t, env, "events"); rows != 0 {
		t.Fatalf("aborted run left %d rows", rows)
	}
	_ = run.sink.Close(context.Background())
}

// TestFailedWriteClosesClean fails a write, which never reaches the catalog,
// and checks that the session still aborts and closes without error.
func TestFailedWriteClosesClean(t *testing.T) {
	env := newTestEnv(t)
	run := openAppendRun(t, env)
	run.apply(t, 1)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	batch := idBatch(t, 1000, 1000)
	_, err := run.sink.Apply(cancelled, batch, filament.ApplyOptions{Policy: run.policy})
	batch.Release()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Apply on a cancelled context = %v", err)
	}
	if err := run.sink.Abort(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := run.sink.Close(context.Background()); err != nil {
		t.Fatalf("Close after a failed write = %v, want clean", err)
	}
	if rows := tableRows(t, env, "events"); rows != 0 {
		t.Fatalf("failed write left %d rows", rows)
	}
}

// TestAbortSurvivesAFailedDataFileDelete removes the written files out from
// under the sink, so Abort cannot delete them, and checks that Abort and Close
// still succeed: an unreferenced data file says nothing about the table.
func TestAbortSurvivesAFailedDataFileDelete(t *testing.T) {
	rotateEveryBatch(t)
	env := newTestEnv(t)
	run := openAppendRun(t, env)
	run.apply(t, 4)
	written := parquetFiles(t, env)
	if len(written) == 0 {
		t.Fatal("no data files written before Abort")
	}
	for _, file := range written {
		if err := os.Remove(file); err != nil {
			t.Fatal(err)
		}
	}
	if err := run.sink.Abort(context.Background()); err != nil {
		t.Fatalf("Abort with undeletable data files = %v", err)
	}
	if err := run.sink.Close(context.Background()); err != nil {
		t.Fatalf("Close after that Abort = %v, want clean", err)
	}
}
