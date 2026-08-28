package postgres

import (
	"testing"
)

func TestIncrementalCursorRequiresTimestamp(t *testing.T) {
	for _, typ := range []string{"timestamp with time zone", "timestamp without time zone", "timestamp(3) with time zone", "timestamptz", "timestamptz(6)"} {
		if !isTimestampType(typ) {
			t.Errorf("%q should be eligible", typ)
		}
	}
	for _, typ := range []string{"bigint", "numeric(12,2)", "uuid", "text", "date", "jsonb"} {
		if isTimestampType(typ) {
			t.Errorf("%q should not be eligible", typ)
		}
	}
}

func TestIncrementalBoundsUsesCompoundStrictCursor(t *testing.T) {
	cols := []pkColumn{{name: "updated_at", typ: "timestamptz"}, {name: "tenant", typ: "text"}, {name: "id", typ: "bigint"}}
	where, args := incrementalBounds(cols, []string{"2026-08-01T00:00:00Z", "a", "4"}, []string{"2026-08-03T00:00:00Z", "z", "9"})
	if len(args) != 6 {
		t.Fatalf("args = %v", args)
	}
	if where == "" {
		t.Fatal("empty bounds")
	}
	_, args = incrementalBounds(cols, []string{"2026-07-31T23:55:00Z"}, []string{"2026-08-03T00:00:00Z", "z", "9"})
	if len(args) != 4 {
		t.Fatalf("lookback args = %v", args)
	}
}

func TestIncrementalPageSQL(t *testing.T) {
	cols := []pkColumn{{name: "updated_at", typ: "timestamptz"}, {name: "id", typ: "bigint"}}
	const qualified = `"public"."users"`
	const where = `t."updated_at" IS NOT NULL`

	q := incrementalPageSQL(qualified, `t."id", t."updated_at", t."name"`, cols, where, 1000)
	want := `SELECT t."id", t."updated_at", t."name" FROM "public"."users" t WHERE t."updated_at" IS NOT NULL ORDER BY t."updated_at", t."id" LIMIT 1000`
	if q != want {
		t.Fatalf("incremental query\n got %q\nwant %q", q, want)
	}
}
