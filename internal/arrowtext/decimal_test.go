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
