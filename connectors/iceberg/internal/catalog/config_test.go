package catalog

import (
	"testing"

	"github.com/galaxy-io/filament"
)

func TestResolveTableUpdateMode(t *testing.T) {
	for _, tc := range []struct {
		table map[string]any
		want  string
	}{
		{table: nil, want: UpdateModeMergeOnRead},
		{table: map[string]any{"update_mode": ""}, want: UpdateModeMergeOnRead},
		{table: map[string]any{"update_mode": UpdateModeCopyOnWrite}, want: UpdateModeCopyOnWrite},
	} {
		setup, err := Resolve(genericConfig(tc.table))
		if err != nil {
			t.Fatal(err)
		}
		if setup.TableUpdateMode != tc.want {
			t.Fatalf("%v: update mode = %q, want %q", tc.table, setup.TableUpdateMode, tc.want)
		}
	}
	if _, err := Resolve(genericConfig(map[string]any{"update_mode": "eventually"})); err == nil {
		t.Fatal("unknown update mode accepted")
	}
}

func genericConfig(table map[string]any) filament.Config {
	cfg := map[string]any{"catalog": map[string]any{"provider": catalogProviderGeneric, "type": "sql"}}
	if table != nil {
		cfg["table"] = table
	}
	return filament.NewConfig(cfg)
}
