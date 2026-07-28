package naming

import (
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"dev-gx", "dev_gx"},
		{"Dev GX (prod!)", "dev_gx_prod"},
		{"already_fine", "already_fine"},
		{"UPPER", "upper"},
		{"dev--gx", "dev_gx"},
		{"dev__gx", "dev_gx"},
		{"--dev-gx--", "dev_gx"},
		{"42warehouse", "_42warehouse"},
		{"café-données", "caf_donn_es"},
		{"", ""},
		{"---", ""},
		{"!!!", ""},
		{strings.Repeat("a", 100), strings.Repeat("a", 63)},
		{strings.Repeat("a", 62) + "-b", strings.Repeat("a", 62)},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
