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
	query, args := listRunsQuery(filament.RunFilter{Search: `dev%`})
	if !strings.Contains(query, "ILIKE '%' || $2 || '%' ESCAPE '\\'") {
		t.Fatalf("query does not use escaped substring search: %s", query)
	}
	if len(args) != 2 || args[0] != `dev%` || args[1] != `dev\%` {
		t.Fatalf("args = %#v", args)
	}
}
