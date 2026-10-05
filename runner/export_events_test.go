package runner

import (
	"reflect"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
)

func TestSourceObserverPublishesTypedExportEvents(t *testing.T) {
	bus := inproc.New()
	t.Cleanup(func() { _ = bus.Close() })
	sub, err := bus.Subscribe(events.AllPattern(), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	e := newEmitter(t.Context(), bus, nil, "tenant", "run")
	report := sourceObserver(e)
	job := filament.ExportJobProgress{CorrelationID: "opaque", Polls: 12, Elapsed: time.Minute, Resumed: true}
	cases := []struct {
		kind filament.SourceProgressKind
		name string
		data any
	}{
		{filament.SourceProgressExportJobCreated, events.ExportJobCreated.Name(), events.ExportJobCreatedEvent(job)},
		{filament.SourceProgressExportJobPolled, events.ExportJobPolled.Name(), events.ExportJobPolledEvent(job)},
		{filament.SourceProgressExportJobReady, events.ExportJobReady.Name(), events.ExportJobReadyEvent(job)},
	}
	for i, tc := range cases {
		report(filament.SourceProgress{Kind: tc.kind, Resource: "items", ExportJob: job})
		select {
		case msg := <-sub.C():
			fact, err := events.Decode(msg)
			if err != nil {
				t.Fatal(err)
			}
			_ = msg.Ack()
			if fact.Name != tc.name || fact.Tenant != "tenant" || fact.Run != "run" || fact.Resource != "items" || fact.Seq != uint64(i+1) || fact.At.IsZero() {
				t.Fatalf("incorrect envelope: %#v", fact)
			}
			// Exercise catalog decoding as used at a transport edge, not just
			// the in-process bus that passes payloads by reference.
			raw, err := events.Marshal(fact)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := events.Unmarshal(raw)
			if err != nil || !reflect.DeepEqual(decoded.Data, tc.data) {
				t.Fatalf("payload round trip: %#v, %v", decoded, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("missing event %s", tc.name)
		}
	}
	if e.runRecords != 0 || e.runBytes != 0 || len(e.res) != 0 {
		t.Fatal("observational events changed sink progress")
	}
}
