package sqlite

import (
	"context"
	"database/sql"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/pressly/goose/v3"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestPipelineExecutionMigrationAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pipelines.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	old := fstest.MapFS{}
	files, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.Name() == "00005_stream_runtime.sql" {
			continue
		}
		data, err := migrations.ReadFile("migrations/" + file.Name())
		if err != nil {
			t.Fatal(err)
		}
		old[file.Name()] = &fstest.MapFile{Data: data}
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, old)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO pipelines (id,tenant_id,name,description,created_at,updated_at) VALUES ('p','t','legacy','',1,1)"); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	store := New(db)
	p, err := store.LoadPipeline(ctx, "t", "p")
	if err != nil || p.ExecutionMode != ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED {
		t.Fatalf("legacy migration: %v %v", p, err)
	}
	p.ExecutionMode = ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS
	if _, err := store.UpdatePipeline(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	p, err = reopened.LoadPipeline(ctx, "t", "p")
	if err != nil || p.ExecutionMode != ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS {
		t.Fatalf("reopen: %v %v", p, err)
	}
}
