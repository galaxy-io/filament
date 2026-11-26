package cron

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestParseLargeStep(t *testing.T) {
	for _, step := range []int{60, math.MaxInt - 1, math.MaxInt} {
		for _, field := range []string{"2/%d", "2-59/%d"} {
			expr := fmt.Sprintf(field, step) + " * * * *"
			t.Run(expr, func(t *testing.T) {
				s, err := Parse(expr)
				if err != nil {
					t.Fatal(err)
				}
				if s.minute != 1<<2 {
					t.Fatalf("minute bits = %064b, want only minute 2", s.minute)
				}
				after := time.Date(2026, time.January, 1, 0, 2, 0, 0, time.UTC)
				want := after.Add(time.Hour)
				if next, ok := s.Next(after); !ok || !next.Equal(want) {
					t.Fatalf("Next = %v, %v; want %v, true", next, ok, want)
				}
			})
		}
	}
}

func TestParseFieldSteps(t *testing.T) {
	for _, tc := range []struct {
		spec string
		min  int
		max  int
		want uint64
	}{
		{"*/15", 0, 59, 1 | 1<<15 | 1<<30 | 1<<45},
		{"2-8/3", 0, 59, 1<<2 | 1<<5 | 1<<8},
		{"58/1", 0, 59, 1<<58 | 1<<59},
		{"*/30", 1, 31, 1<<1 | 1<<31},
		{"1-3/60", 1, 31, 1 << 1},
		{"59/60", 0, 59, 1 << 59},
		{fmt.Sprintf("*/%d", math.MaxInt), 1, 31, 1 << 1},
		{fmt.Sprintf("2/%d,5", math.MaxInt), 0, 59, 1<<2 | 1<<5},
	} {
		t.Run(tc.spec, func(t *testing.T) {
			got, err := parseField(tc.spec, tc.min, tc.max)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("bits = %064b, want %064b", got, tc.want)
			}
		})
	}
}
