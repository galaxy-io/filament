package mysql

import (
	"math"
	"slices"
	"strconv"
	"testing"

	"github.com/galaxy-io/filament/connectors/internal/dbconfig"
)

func TestIntBoundariesStayOrderedForWideKeys(t *testing.T) {
	cases := []struct {
		name   string
		lo, hi int64
		k      int
	}{
		{"small range", 1, 1000, 8},
		{"snowflake ids", 1, 1_800_000_000_000_000_000, 8},
		{"whole int64 range", math.MinInt64, math.MaxInt64, 64},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bounds := intBoundaries(c.lo, c.hi, c.k)
			if len(bounds) != c.k-1 {
				t.Fatalf("got %d boundaries, want %d", len(bounds), c.k-1)
			}
			prev := c.lo
			for i, s := range bounds {
				b, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					t.Fatalf("boundary %d = %q: %v", i, s, err)
				}
				if b <= prev || b > c.hi {
					t.Fatalf("boundary %d = %d, want in (%d, %d]", i, b, prev, c.hi)
				}
				prev = b
			}
		})
	}
}

func TestReverseTunnelHostAddsTheMethod(t *testing.T) {
	methods := func(connector *Source) []string {
		var values []string
		for _, field := range connector.Spec().Config.Fields {
			if field.Name != dbconfig.TunnelField {
				continue
			}
			for _, option := range field.Fields[0].Enum {
				values = append(values, option.Value)
			}
		}
		return values
	}
	connector := New()
	if got := methods(connector); slices.Contains(got, dbconfig.TunnelReverse) || !slices.Contains(got, dbconfig.TunnelSSH) {
		t.Fatalf("methods without a host = %v", got)
	}
	connector.UseReverseTunnelHost("tunnel.internal")
	if got := methods(connector); !slices.Contains(got, dbconfig.TunnelReverse) {
		t.Fatalf("methods with a host = %v", got)
	}
}
