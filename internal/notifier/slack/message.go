package slack

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/notifier"
)

// message is an incoming webhook body. The attachment carries the status
// color; its fallback is the text shown in notifications.
type message struct {
	Attachments []attachment `json:"attachments"`
}

type attachment struct {
	Color    string  `json:"color"`
	Fallback string  `json:"fallback"`
	Blocks   []block `json:"blocks"`
}

type block struct {
	Type     string `json:"type"`
	Text     *text  `json:"text,omitempty"`
	Elements []text `json:"elements,omitempty"`
}

type text struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// event reads the fields any exported run event may carry.
type event struct {
	RunID     string    `json:"run_id"`
	At        time.Time `json:"at"`
	Records   int64     `json:"records"`
	Bytes     int64     `json:"bytes"`
	Committed bool      `json:"committed"`
}

const (
	green = "#2EB67D"
	red   = "#E01E5A"
	amber = "#ECB22E"
	grey  = "#9AA0A6"
)

var statuses = map[ingestionv1.NotifierEvent]struct{ color, label string }{
	ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_STARTED:   {grey, "started"},
	ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_COMPLETED: {green, "completed"},
	ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_FAILED:    {red, "failed"},
	ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_PARTIAL:   {amber, "partially completed"},
	ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_CANCELED:  {grey, "canceled"},
	ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_PAUSED:    {grey, "paused"},
}

const separator = " · "

// render builds the message for one notification. Every exported event kind
// must have a status.
func render(n notifier.Notification) (message, error) {
	status, ok := statuses[n.TriggerEvent]
	if !ok {
		return message{}, fmt.Errorf("slack: %s has no message", n.TriggerType)
	}
	var e event
	if err := json.Unmarshal(n.Event, &e); err != nil {
		return message{}, fmt.Errorf("slack: decode %s event: %w", n.TriggerType, err)
	}
	name := n.PipelineName
	if name == "" {
		name = n.PipelineID
	}

	summary := "Filament run " + status.label
	switch n.TriggerEvent {
	case ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_COMPLETED:
		summary += separator + formatRecords(e.Records) + separator + formatBytes(e.Bytes)
	case ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_PAUSED:
		if e.Committed {
			summary += separator + "committed"
		} else {
			summary += separator + "not committed"
		}
	}

	return message{Attachments: []attachment{{
		Color:    status.color,
		Fallback: "Filament run " + status.label + separator + name,
		Blocks: []block{
			{Type: "section", Text: mrkdwn("*" + escape(name) + "*\n" + summary)},
			{Type: "context", Elements: []text{*mrkdwn(escape(e.RunID) + separator + timestamp(e.At))}},
		},
	}}}, nil
}

func mrkdwn(s string) *text { return &text{Type: "mrkdwn", Text: s} }

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// escape neutralizes the three characters Slack treats as control characters.
func escape(s string) string { return escaper.Replace(s) }

// timestamp renders in each reader's timezone, with UTC as the fallback.
func timestamp(at time.Time) string {
	return "<!date^" + strconv.FormatInt(at.Unix(), 10) + "^{date_short_pretty} at {time}|" + at.UTC().Format("2006-01-02 15:04 UTC") + ">"
}

// formatRecords renders a count with its unit, such as 1 record.
func formatRecords(n int64) string {
	if n == 1 {
		return "1 record"
	}
	return formatCount(n) + " records"
}

// formatCount groups digits in threes, such as 1,204,331.
func formatCount(n int64) string {
	digits := strconv.FormatInt(n, 10)
	sign := ""
	if n < 0 {
		sign, digits = "-", digits[1:]
	}
	var b strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(d)
	}
	return sign + b.String()
}

// formatBytes renders a size in decimal units, such as 412.3 MB.
func formatBytes(n int64) string {
	if n < 1000 {
		return strconv.FormatInt(n, 10) + " B"
	}
	value, unit := float64(n), 0
	for value >= 1000 && unit < len(units) {
		value /= 1000
		unit++
	}
	return strconv.FormatFloat(value, 'f', 1, 64) + " " + units[unit-1]
}

var units = []string{"kB", "MB", "GB", "TB", "PB", "EB"}
