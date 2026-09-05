// Package style paints CLI output with one accent and background-aware tints.
package style

import (
	"fmt"
	"image/color"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"golang.org/x/term"
)

const (
	// Indent precedes every table row.
	Indent = "  "
	// Gutter separates table columns.
	Gutter = 5
)

// Colors is the palette for one background. The run tones are the UI's icon
// tokens for each run state, so a status reads the same on both surfaces.
type Colors struct {
	Accent, Label, Muted, Success, Error int32

	Completed, Failed, Partial, Running, Requested, Scheduled, Paused, Canceled int32
}

// Palette returns the tints that read on a dark or light background.
func Palette(dark bool) Colors {
	if dark {
		return Colors{
			Accent: 0x5b92e7, Label: 0xa3a3a2, Muted: 0x666665, Success: 0x6ee4b0, Error: 0xfb7185,
			Completed: 0x4cb782, Failed: 0xf6525c, Partial: 0xf3a8e5, Running: 0x99cffd,
			Requested: 0xffd2a8, Scheduled: 0xfff7aa, Paused: 0xbde8ee, Canceled: 0xacacac,
		}
	}
	return Colors{
		Accent: 0x5b92e7, Label: 0x5c5c5b, Muted: 0x8a8a89, Success: 0x1a8a5a, Error: 0xc0392b,
		Completed: 0x7ccfaa, Failed: 0xf65166, Partial: 0xf77fb4, Running: 0x7dabff,
		Requested: 0xeeab72, Scheduled: 0xd9cf74, Paused: 0x5cc6bb, Canceled: 0x3f3f3f,
	}
}

// Hex converts a palette entry to a lipgloss colour.
func Hex(value int32) color.Color {
	return lipgloss.Color(fmt.Sprintf("#%06x", value))
}

var (
	darkOnce   sync.Once
	darkResult = true
)

// Dark reports whether the terminal behind in and out has a dark background.
// The terminal is queried once per process; non-terminals are assumed dark.
func Dark(in io.Reader, out io.Writer) bool {
	inFile, inOK := in.(*os.File)
	outFile, outOK := out.(*os.File)
	if !inOK || !outOK || !term.IsTerminal(int(inFile.Fd())) || !term.IsTerminal(int(outFile.Fd())) {
		return true
	}
	darkOnce.Do(func() { darkResult = lipgloss.HasDarkBackground(inFile, outFile) })
	return darkResult
}

// Painter styles text for one destination. The zero value paints nothing.
type Painter struct {
	enabled bool
	colors  Colors

	accent, bold, label, muted, success, failure lipgloss.Style
}

// New inspects w, NO_COLOR, and the terminal background.
func New(w io.Writer) Painter {
	file, ok := w.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return Painter{}
	}
	return Styled(Dark(os.Stdin, file))
}

// Styled builds a painter for a known background without probing a terminal.
func Styled(dark bool) Painter {
	colors := Palette(dark)
	return Painter{
		enabled: true,
		colors:  colors,
		accent:  lipgloss.NewStyle().Foreground(Hex(colors.Accent)),
		bold:    lipgloss.NewStyle().Bold(true),
		label:   lipgloss.NewStyle().Foreground(Hex(colors.Label)),
		muted:   lipgloss.NewStyle().Foreground(Hex(colors.Muted)),
		success: lipgloss.NewStyle().Foreground(Hex(colors.Success)),
		failure: lipgloss.NewStyle().Foreground(Hex(colors.Error)),
	}
}

// Enabled reports whether output is styled.
func (p Painter) Enabled() bool { return p.enabled }

// Accent paints text in the brand colour.
func (p Painter) Accent(text string) string { return p.accent.Render(text) }

// Bold paints headings and primary cells.
func (p Painter) Bold(text string) string { return p.bold.Render(text) }

// Label paints secondary text such as keys.
func (p Painter) Label(text string) string { return p.label.Render(text) }

// Muted paints tertiary text such as placeholders and helper lines.
func (p Painter) Muted(text string) string { return p.muted.Render(text) }

// Success paints completed states.
func (p Painter) Success(text string) string { return p.success.Render(text) }

// Error paints failures.
func (p Painter) Error(text string) string { return p.failure.Render(text) }

// Status paints text in the run tone for status, e.g. the failed red.
func (p Painter) Status(status, text string) string { return p.status(status).Render(text) }

// Title renders a context line such as "> Sources in filament.yaml".
func (p Painter) Title(prefix, subject string) string {
	line := p.Accent(">") + " " + prefix
	if subject != "" {
		line += " " + p.Bold(subject)
	}
	return line
}

// Role decides how a column's cells are emphasised.
type Role int

// Column roles, from least to most emphasis. RoleStatus colors a "● Status"
// cell by what the status means.
const (
	RolePlain Role = iota
	RolePrimary
	RoleSecondary
	RoleMuted
	RoleNumber
	RoleStatus
)

// Layout is how a table is drawn.
type Layout int

// Table layouts: boxed is dado's ruled box, plain is a Vercel-style
// whitespace grid.
const (
	LayoutBoxed Layout = iota
	LayoutPlain
)

// String names the layout the way ParseLayout reads it.
func (l Layout) String() string {
	if l == LayoutPlain {
		return "plain"
	}
	return "boxed"
}

// ParseLayout reads a layout name from a flag or environment value.
func ParseLayout(value string) (Layout, error) {
	switch value {
	case "", "boxed":
		return LayoutBoxed, nil
	case "plain":
		return LayoutPlain, nil
	default:
		return LayoutBoxed, fmt.Errorf("layout must be boxed or plain, not %q", value)
	}
}

// Column describes one table column.
type Column struct {
	Title string
	Role  Role
}

// Grid renders rows in the given layout.
func (p Painter) Grid(layout Layout, columns []Column, rows [][]string) string {
	if layout == LayoutPlain {
		return p.plainTable(columns, rows)
	}
	return p.Table(columns, rows)
}

// plainTable renders a whitespace grid: no rules, a two-space margin, five
// spaces between columns, muted headers, and role-styled cells.
func (p Painter) plainTable(columns []Column, rows [][]string) string {
	headers := make([]string, len(columns))
	for index, column := range columns {
		headers[index] = column.Title
	}
	widths := Widths(append([][]string{headers}, rows...))
	const gap = "     "
	var out strings.Builder
	line := func(cells []string, paint func(int, string) string) {
		out.WriteString(Indent)
		for index, cell := range cells {
			out.WriteString(paint(index, cell))
			if index < len(cells)-1 {
				out.WriteString(strings.Repeat(" ", widths[index]-Width(cell)) + gap)
			}
		}
		out.WriteString("\n")
	}
	line(headers, func(_ int, text string) string { return p.muted.Render(text) })
	for _, row := range rows {
		line(row, func(index int, text string) string { return p.cell(columns[index].Role, text).Render(text) })
	}
	return out.String()
}

// cell picks the style for one body cell by its column role.
func (p Painter) cell(role Role, text string) lipgloss.Style {
	switch role {
	case RolePrimary:
		return p.bold
	case RoleSecondary:
		return p.label
	case RoleMuted:
		return p.muted
	case RoleStatus:
		return p.status(text)
	default:
		return lipgloss.NewStyle()
	}
}

// runTones pairs a status word with its palette tone, in match order.
var runTones = []struct {
	word string
	tone func(Colors) int32
}{
	{"complete", func(c Colors) int32 { return c.Completed }},
	{"ready", func(c Colors) int32 { return c.Completed }},
	{"fail", func(c Colors) int32 { return c.Failed }},
	{"partial", func(c Colors) int32 { return c.Partial }},
	{"running", func(c Colors) int32 { return c.Running }},
	{"requested", func(c Colors) int32 { return c.Requested }},
	{"scheduled", func(c Colors) int32 { return c.Scheduled }},
	{"paused", func(c Colors) int32 { return c.Paused }},
	{"cancel", func(c Colors) int32 { return c.Canceled }},
}

// status colors a status cell with the UI's tone for that run state; anything
// unrecognised is muted.
func (p Painter) status(text string) lipgloss.Style {
	lower := strings.ToLower(text)
	for _, entry := range runTones {
		if !strings.Contains(lower, entry.word) {
			continue
		}
		if !p.enabled {
			return lipgloss.NewStyle()
		}
		return lipgloss.NewStyle().Foreground(Hex(entry.tone(p.colors)))
	}
	return p.muted
}

// Table renders a boxed table in dado's style: rounded corners, column rules,
// a header rule, an accent header, and role-styled body cells.
func (p Painter) Table(columns []Column, rows [][]string) string {
	headers := make([]string, len(columns))
	for index, column := range columns {
		headers[index] = column.Title
	}
	grid := table.New().
		Headers(headers...).
		Rows(rows...).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(p.muted).
		BorderHeader(true).
		BorderColumn(true).
		BorderRow(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			cell := lipgloss.NewStyle().Padding(0, 1)
			if columns[col].Role == RoleNumber {
				cell = cell.Align(lipgloss.Right)
			}
			if row == table.HeaderRow {
				return cell.Inherit(p.accent).Bold(p.enabled)
			}
			text := ""
			if row >= 0 && row < len(rows) && col < len(rows[row]) {
				text = rows[row][col]
			}
			return cell.Inherit(p.cell(columns[col].Role, text))
		})
	var out strings.Builder
	for _, line := range strings.Split(grid.Render(), "\n") {
		out.WriteString(Indent + strings.TrimRight(line, " ") + "\n")
	}
	return out.String()
}

// Width measures text without its styling.
func Width(text string) int {
	return lipgloss.Width(text)
}

// Widths returns the widest cell per column.
func Widths(rows [][]string) []int {
	widths := []int{}
	for _, row := range rows {
		for index, cell := range row {
			if index >= len(widths) {
				widths = append(widths, 0)
			}
			widths[index] = max(widths[index], Width(cell))
		}
	}
	return widths
}

// Count renders 1234567 as 1.2M.
func Count(value int64) string {
	switch {
	case value >= 1_000_000_000:
		return trimZero(fmt.Sprintf("%.1f", float64(value)/1_000_000_000)) + "B"
	case value >= 1_000_000:
		return trimZero(fmt.Sprintf("%.1f", float64(value)/1_000_000)) + "M"
	case value >= 1_000:
		return trimZero(fmt.Sprintf("%.1f", float64(value)/1_000)) + "K"
	default:
		return fmt.Sprintf("%d", value)
	}
}

// Bytes renders a byte count as KB, MB, or GB; zero is a dash.
func Bytes(value int64) string {
	const unit = 1024
	switch {
	case value <= 0:
		return "–"
	case value >= unit*unit*unit:
		return trimZero(fmt.Sprintf("%.1f", float64(value)/(unit*unit*unit))) + " GB"
	case value >= unit*unit:
		return trimZero(fmt.Sprintf("%.1f", float64(value)/(unit*unit))) + " MB"
	case value >= unit:
		return trimZero(fmt.Sprintf("%.1f", float64(value)/unit)) + " KB"
	default:
		return fmt.Sprintf("%d B", value)
	}
}

func trimZero(text string) string {
	return strings.TrimSuffix(text, ".0")
}

// Elapsed renders a duration the way a status line wants it: 12ms, 1.3s, 39s.
func Elapsed(value time.Duration) string {
	switch {
	case value < time.Millisecond:
		return "<1ms"
	case value < time.Second:
		return value.Round(time.Millisecond).String()
	case value < time.Minute:
		return value.Round(100 * time.Millisecond).String()
	default:
		return value.Round(time.Second).String()
	}
}
