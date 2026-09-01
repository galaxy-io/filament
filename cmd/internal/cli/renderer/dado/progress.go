package dado

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/atterpac/dado/inline"
	"github.com/gdamore/tcell/v2"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/cmd/internal/cli/style"
)

type runRowState int

const (
	rowPending runRowState = iota
	rowRunning
	rowDone
	rowFailed
	rowCancelled
)

type runRow struct {
	name    string
	records int64
	bytes   int64
	state   runRowState
	err     string
}

var runColumns = []style.Column{{Title: "Resource", Role: style.RolePrimary}, {Title: "Rows", Role: style.RoleNumber}, {Title: "Status"}}

// runProgressView draws the live run as a grid of resources.
type runProgressView struct {
	rows  []*runRow
	theme inline.InlineTheme
	tick  int
}

func (v *runProgressView) Frame(width int) *inline.Frame {
	rows := v.cells()
	widths := style.Widths(append([][]string{{"Resource", "Rows", "Status"}}, rows...))
	// Row 0 stays blank so the grid sits one line below the discovery notice.
	frame := inline.NewFrame(width, len(v.rows)+2)
	x := utf8.RuneCountInString(style.Indent)
	for index, column := range runColumns {
		draw(frame, x, 1, column.Title, v.theme.Accent.Bold(true))
		x += widths[index] + style.Gutter
	}
	for index, row := range v.rows {
		y := index + 2
		x = utf8.RuneCountInString(style.Indent)
		draw(frame, x, y, row.name, v.theme.Text.Bold(true))
		x += widths[0] + style.Gutter
		count := rows[index][1]
		draw(frame, x+widths[1]-utf8.RuneCountInString(count), y, count, v.theme.Text)
		x += widths[1] + style.Gutter
		marker, markerStyle := v.marker(row)
		x = draw(frame, x, y, marker+" ", markerStyle)
		draw(frame, x, y, v.word(row), v.wordStyle(row))
	}
	return frame
}

// lines renders the final rows for scrollback, matching the live layout.
func (v *runProgressView) lines(p style.Painter) string {
	rows := v.cells()
	widths := style.Widths(append([][]string{{"Resource", "Rows", "Status"}}, rows...))
	out := make([]string, 0, len(v.rows)+1)
	out = append(out, style.Indent+p.Accent(pad("Resource", widths[0]))+gutter()+p.Accent(pad("Rows", widths[1]))+gutter()+p.Accent("Status"))
	for index, row := range v.rows {
		marker, _ := v.marker(row)
		painted := p.Muted(marker)
		word := v.word(row)
		switch row.state {
		case rowDone:
			painted = p.Success(marker)
		case rowFailed:
			painted = p.Error(marker)
			word = p.Error(word)
		case rowCancelled, rowPending:
			word = p.Muted(word)
		}
		count := rows[index][1]
		padding := strings.Repeat(" ", widths[1]-utf8.RuneCountInString(count))
		out = append(out, style.Indent+p.Bold(pad(row.name, widths[0]))+gutter()+padding+count+gutter()+painted+" "+word)
	}
	return strings.Join(out, "\n")
}

func gutter() string {
	return strings.Repeat(" ", style.Gutter)
}

func pad(text string, width int) string {
	return text + strings.Repeat(" ", max(width-utf8.RuneCountInString(text), 0))
}

func (v *runProgressView) cells() [][]string {
	rows := make([][]string, 0, len(v.rows))
	for _, row := range v.rows {
		marker, _ := v.marker(row)
		rows = append(rows, []string{row.name, v.count(row), marker + " " + v.word(row)})
	}
	return rows
}

func (v *runProgressView) wordStyle(row *runRow) tcell.Style {
	switch row.state {
	case rowFailed:
		return v.theme.Error
	case rowCancelled, rowPending:
		return v.theme.Muted
	default:
		return v.theme.Text
	}
}

func (v *runProgressView) count(row *runRow) string {
	if row.state == rowRunning || row.state == rowDone || row.records > 0 {
		return style.Count(row.records)
	}
	return "–"
}

func (v *runProgressView) marker(row *runRow) (string, tcell.Style) {
	switch row.state {
	case rowRunning:
		frames := v.theme.Status.SpinnerFrames
		return frames[v.tick%len(frames)], v.theme.Accent
	case rowDone:
		return v.theme.Status.SuccessMarker, v.theme.Success
	case rowFailed:
		return v.theme.Status.FailureMarker, v.theme.Error
	case rowCancelled:
		return v.theme.Status.CancelledMarker, v.theme.Muted
	default:
		return v.theme.Status.PendingMarker, v.theme.Muted
	}
}

func (v *runProgressView) word(row *runRow) string {
	switch row.state {
	case rowRunning:
		return "Running"
	case rowDone:
		return "Done"
	case rowFailed:
		return row.err
	case rowCancelled:
		return "Cancelled"
	default:
		return "Waiting"
	}
}

func runSummaryLine(p style.Painter, rows int, result model.RunResult, elapsed time.Duration, runErr error) string {
	if runErr != nil {
		return p.Error("✗ run failed") + " " + p.Muted("· "+runErr.Error())
	}
	return p.Success("✓ synced") + fmt.Sprintf(" %d resources · %s rows · %s", rows, style.Count(result.Records), humanDuration(elapsed))
}
