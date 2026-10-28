package arrowtext

import (
	"testing"

	"github.com/apache/arrow-go/v18/arrow/decimal128"
)

func TestAppendDecimal(t *testing.T) {
	cases := map[string]struct {
		v     decimal128.Num
		scale int
	}{
		"1.50":                 {decimal128.FromI64(150), 2},
		"-0.05":                {decimal128.FromI64(-5), 2},
		"0.00":                 {decimal128.FromI64(0), 2},
		"7":                    {decimal128.FromI64(7), 0},
		"9223372036854775808":  {decimal128.New(0, 1<<63), 0},
		"18446744073709551616": {decimal128.New(1, 0), 0},
	}
	for want, test := range cases {
		if got := string(AppendDecimal(nil, test.v, test.scale)); got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}

func TestAppendDecimalNegativeScale(t *testing.T) {
	cases := []struct {
		name  string
		v     decimal128.Num
		scale int
		want  string
	}{
		{"positive", decimal128.FromI64(1234), -2, "123400"},
		{"negative", decimal128.FromI64(-5), -1, "-50"},
		{"zero", decimal128.FromI64(0), -2, "0"},
		{"wide", decimal128.New(1, 0), -2, "1844674407370955161600"},
		{"large scale", decimal128.FromI64(1), -38, "100000000000000000000000000000000000000"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			for _, prefix := range []string{"", "prefix:"} {
				if got := string(AppendDecimal([]byte(prefix), test.v, test.scale)); got != prefix+test.want {
					t.Errorf("got %q want %q", got, prefix+test.want)
				}
			}
		})
	}
}
