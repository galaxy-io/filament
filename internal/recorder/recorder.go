// Package recorder taps the event bus and writes every matching event as one
// NDJSON line — a timestamped, ordered timeseries of a run's facts for inspection
// (jq, plotting, audit). It is a read-only observer: an ephemeral subscription that
// never acks state or mutates anything. Order lines by `seq` for a run's total order;
// `ts` is the emit wall-clock.
package recorder

import (
	"bufio"
	"encoding/json"
	"io"
	"time"

	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament"
)

// line is the flat, jq-friendly shape of one recorded event. Payload fields are
// omitempty so each line carries only what its event type set.
type line struct {
	TS       string `json:"ts"`
	Seq      uint64 `json:"seq"`
	Type     string `json:"type"`
	Tenant   string `json:"tenant,omitempty"`
	Run      string `json:"run,omitempty"`
	Resource string `json:"resource,omitempty"`
	Records  int64  `json:"records,omitempty"`
	Bytes    int64  `json:"bytes,omitempty"`
	CRC      uint32         `json:"crc,omitempty"`
	URI      string         `json:"uri,omitempty"`
	Error    string         `json:"error,omitempty"`
	Cursor   map[string]any `json:"cursor,omitempty"` // checkpoint delta on checkpoint/batch facts
}

func toLine(ev ingestion.Event) line {
	l := line{
		TS:       ev.At.UTC().Format(time.RFC3339Nano),
		Seq:      ev.Seq,
		Type:     ev.Type.String(),
		Tenant:   string(ev.Tenant),
		Run:      string(ev.Run),
		Resource: ev.Resource,
		Records:  ev.Fields.Records,
		Bytes:    ev.Fields.Bytes,
		CRC:      ev.Fields.CRC,
		URI:      ev.Fields.URI,
		Error:    ev.Fields.Error,
	}
	if cp := ev.Fields.Checkpoint; cp != nil {
		l.Cursor = cp.Cursor
	}
	return l
}

// Recorder is a live bus tap writing NDJSON. Start it before the work begins (the
// in-proc bus is fire-and-forget — it has no backlog to replay), then Stop after.
type Recorder struct {
	sub  eventbus.Subscription
	done chan struct{}
	n    int
}

// Start subscribes to pattern (e.g. ingestion.TenantPattern / ingestion.RunPattern) and streams
// every matching event to w as NDJSON until Stop. A drain goroutine owns w.
func Start(bus eventbus.Bus, pattern string, w io.Writer) (*Recorder, error) {
	sub, err := bus.Subscribe(pattern, eventbus.SubOpts{})
	if err != nil {
		return nil, err
	}
	r := &Recorder{sub: sub, done: make(chan struct{})}
	go func() {
		defer close(r.done)
		bw := bufio.NewWriter(w)
		defer func() { _ = bw.Flush() }()
		enc := json.NewEncoder(bw)
		for msg := range sub.C() {
			ev, err := ingestion.EventOf(msg)
			if err != nil {
				continue
			}
			_ = enc.Encode(toLine(ev))
			r.n++
		}
	}()
	return r, nil
}

// Stop closes the subscription, waits for the drain to flush, and returns the number
// of events written. Call it after the work has finished and its tail has drained.
func (r *Recorder) Stop() int {
	_ = r.sub.Close() // closes C(), ending the drain loop
	<-r.done
	return r.n
}
