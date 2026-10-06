package mysql

import (
	"math"
	"strconv"
	"testing"
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
