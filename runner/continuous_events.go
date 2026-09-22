package runner

import (
	"sync"

	"github.com/galaxy-io/filament/events"
)

// continuousPipelineEvents forwards diagnostics as they happen and holds write
// success until the current epoch is certified. Only one epoch is active. Failed
// attempts discard their pending facts; no raw source positions are published.
type continuousPipelineEvents struct {
	emitter *emitter
	mu      sync.Mutex
	pending []events.Fact
}

func (e *continuousPipelineEvents) observe(f events.Fact) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, written := f.Data.(events.BatchWrittenEvent); written {
		e.pending = append(e.pending, f)
		return
	}
	e.publish(f)
}

// publish assigns the sequence at publication, since deferred writes must not
// reuse an earlier sequence than diagnostics already delivered to the bus.
func (e *continuousPipelineEvents) publish(f events.Fact) {
	f.Seq = e.emitter.next()
	_ = e.emitter.publish(f)
}

// certified is called after SealEpoch has joined all writes and CommitEpoch (or
// verified historical lookup) has established durable progress.
func (e *continuousPipelineEvents) certified() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, f := range e.pending {
		e.publish(f)
	}
	clear(e.pending)
	e.pending = e.pending[:0]
}
