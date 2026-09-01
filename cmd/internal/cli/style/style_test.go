package style

import (
	"strings"
	"testing"
)

func TestTableRendersBoxed(t *testing.T) {
	t.Parallel()
	var p Painter
	text := p.Table([]Column{{Title: "Name", Role: RolePrimary}, {Title: "Rows", Role: RoleNumber}}, [][]string{
		{"customers", "1.8M"},
		{"orders", "600K"},
	})
	want := "  ╭───────────┬──────╮\n" +
		"  │ Name      │ Rows │\n" +
		"  ├───────────┼──────┤\n" +
		"  │ customers │ 1.8M │\n" +
		"  │ orders    │ 600K │\n" +
		"  ╰───────────┴──────╯\n"
	if text != want {
		t.Fatalf("table =\n%s\nwant\n%s", text, want)
	}
}

func TestStyledTableKeepsAlignment(t *testing.T) {
	t.Parallel()
	p := Styled(true)
	text := p.Table([]Column{{Title: "Name", Role: RolePrimary}, {Title: "Rows", Role: RoleNumber}}, [][]string{{"customers", "1.8M"}, {"o", "2"}})
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) != 6 {
		t.Fatalf("lines = %d", len(lines))
	}
	for index := 1; index < len(lines); index++ {
		if Width(lines[index]) != Width(lines[0]) {
			t.Fatalf("line %d width %d != %d:\n%q", index, Width(lines[index]), Width(lines[0]), text)
		}
	}
	if !strings.Contains(lines[1], "Name") || !strings.Contains(text, "\x1b[") {
		t.Fatalf("styled table missing header or styling:\n%q", text)
	}
	if Width(p.Accent("abc")) != 3 || Count(1_800_000) != "1.8M" || Count(600_000) != "600K" || Count(42) != "42" {
		t.Fatal("width or count formatting")
	}
}
