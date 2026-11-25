package catalog

import (
	"testing"

	"github.com/galaxy-io/filament"
)

func TestResolveTableLocationRoot(t *testing.T) {
	for _, tc := range []struct {
		table map[string]any
		want  string
	}{
		{table: nil, want: ""},
		{table: map[string]any{"location_root": "s3://bucket/warehouse"}, want: "s3://bucket/warehouse"},
	} {
		setup, err := Resolve(genericConfig(tc.table))
		if err != nil {
			t.Fatal(err)
		}
		if setup.TableLocationRoot != tc.want {
			t.Fatalf("%v: location root = %q, want %q", tc.table, setup.TableLocationRoot, tc.want)
		}
	}
}

func genericConfig(table map[string]any) filament.Config {
	cfg := map[string]any{"catalog": map[string]any{"provider": catalogProviderGeneric, "type": "sql"}}
	if table != nil {
		cfg["table"] = table
	}
	return filament.NewConfig(cfg)
}
