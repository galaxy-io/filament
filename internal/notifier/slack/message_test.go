package slack

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/notifier"
)

func TestRenderCompleted(t *testing.T) {
	at := time.Date(2026, 9, 29, 14, 2, 0, 0, time.UTC)
	m, err := render(notifier.Notification{
		PipelineName: "orders <prod>",
		TriggerType:  "run.completed",
		TriggerEvent: ingestionv1.NotifierEvent_NOTIFIER_EVENT_RUN_COMPLETED,
		Event:        json.RawMessage(`{"type":"run.completed","run_id":"run-1","at":"2026-09-29T14:02:00Z","records":1204331,"bytes":412300000}`),
	})
	if err != nil {
		t.Fatalf("render() = %v", err)
	}
	want := message{Attachments: []attachment{{
		Color:    green,
		Fallback: "Filament run completed · orders <prod>",
		Blocks: []block{
			{Type: "section", Text: mrkdwn("*orders &lt;prod&gt;*\nFilament run completed · 1,204,331 records · 412.3 MB")},
			{Type: "context", Elements: []text{
				*mrkdwn("run-1 · <!date^" + strconv.FormatInt(at.Unix(), 10) + "^{date_short_pretty} at {time}|2026-09-29 14:02 UTC>"),
			}},
		},
	}}}
	got, _ := json.Marshal(m)
	expected, _ := json.Marshal(want)
	if string(got) != string(expected) {
		t.Fatalf("render() =\n%s\nwant\n%s", got, expected)
	}
}

func TestRenderCoversEveryExportedEvent(t *testing.T) {
	for _, d := range notifier.Exported() {
		name := d.Entity + "." + d.Event
		event, err := notifier.ParseEventName(name)
		if err != nil {
			t.Fatalf("ParseEventName(%q) = %v", name, err)
		}
		m, err := render(notifier.Notification{
			PipelineID: "p1", TriggerType: name, TriggerEvent: event,
			Event: json.RawMessage(`{"run_id":"run-1","at":"2026-09-29T14:02:00Z"}`),
		})
		if err != nil || len(m.Attachments) != 1 || m.Attachments[0].Color == "" || len(m.Attachments[0].Blocks) != 2 {
			t.Errorf("render(%s) = %+v, %v", name, m, err)
		}
	}
}

func TestFormat(t *testing.T) {
	for n, want := range map[int64]string{0: "0", 999: "999", 1000: "1,000", 1204331: "1,204,331", -1500: "-1,500"} {
		if got := formatCount(n); got != want {
			t.Errorf("formatCount(%d) = %q, want %q", n, got, want)
		}
	}
	for n, want := range map[int64]string{0: "0 records", 1: "1 record", 6: "6 records"} {
		if got := formatRecords(n); got != want {
			t.Errorf("formatRecords(%d) = %q, want %q", n, got, want)
		}
	}
	for n, want := range map[int64]string{0: "0 B", 999: "999 B", 1000: "1.0 kB", 412300000: "412.3 MB", 30800000000: "30.8 GB"} {
		if got := formatBytes(n); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}
