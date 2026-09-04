package postgres

import (
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
)

func TestEscapeLikePattern(t *testing.T) {
	if got, want := escapeLikePattern(`dev%_\ops`), `dev\%\_\\ops`; got != want {
		t.Fatalf("escapeLikePattern() = %q, want %q", got, want)
	}
}

func TestListRunsQueryUsesLiteralSubstringSearch(t *testing.T) {
	query, args := listRunsQuery(filament.RunFilter{Tenant: "t1", Search: `dev%`})
	if !strings.Contains(query, "ILIKE '%' || $3 || '%' ESCAPE '\\'") {
		t.Fatalf("query does not use escaped substring search: %s", query)
	}
	if len(args) != 3 || args[0] != "t1" || args[1] != `dev%` || args[2] != `dev\%` {
		t.Fatalf("args = %#v", args)
	}
}
