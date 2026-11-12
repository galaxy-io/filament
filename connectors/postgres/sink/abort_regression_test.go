package postgres

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
)

func TestAbortPreservesDurableRows(t *testing.T) {
	for _, mode := range []filament.IngestionType{filament.IngestionFullAppend, filament.IngestionFullUpsert, filament.IngestionCDCMerge, filament.IngestionFullReplace} {
		t.Run(string(mode), func(t *testing.T) {
			ctx, s, _ := driftSink(t, "strict", mode, false)
			if err := s.EnsureSchema(ctx, "items", driftModel()); err != nil {
				t.Fatal(err)
			}
			q := pgx.Identifier{s.schema, "items"}.Sanitize()
			if _, err := s.pool.Exec(ctx, "INSERT INTO "+q+" VALUES (1,'durable')"); err != nil {
				t.Fatal(err)
			}
			if err := s.Begin(ctx); err != nil {
				t.Fatal(err)
			}
			if err := s.Abort(ctx); err != nil {
				t.Fatal(err)
			}
			var rows int
			if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM "+q).Scan(&rows); err != nil {
				t.Fatal(err)
			}
			want := 1
			if mode == filament.IngestionFullReplace {
				want = 0
			}
			if rows != want {
				t.Fatalf("rows=%d want=%d", rows, want)
			}
		})
	}
}

func TestAbortAttemptsEveryReplacement(t *testing.T) {
	ctx, s, _ := driftSink(t, "strict", filament.IngestionFullReplace, false)
	if err := s.EnsureSchema(ctx, "items", driftModel()); err != nil {
		t.Fatal(err)
	}
	q := s.tables["items"].qualified
	if _, err := s.pool.Exec(ctx, "INSERT INTO "+q+" VALUES (1,'partial')"); err != nil {
		t.Fatal(err)
	}
	s.policies[""] = s.policies["items"]
	s.tables["missing_a"] = &table{qualified: pgx.Identifier{s.schema, "missing_a"}.Sanitize()}
	s.tables["missing_b"] = &table{qualified: pgx.Identifier{s.schema, "missing_b"}.Sanitize()}
	if err := s.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	err := s.Abort(ctx)
	if err == nil || !strings.Contains(err.Error(), "missing_a") || !strings.Contains(err.Error(), "missing_b") {
		t.Fatalf("missing cleanup errors: %v", err)
	}
	var rows int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM "+q).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("healthy table cleanup skipped: %d rows", rows)
	}
}
