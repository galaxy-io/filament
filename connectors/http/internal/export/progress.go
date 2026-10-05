package export

import (
	"crypto/rand"
	"time"

	"github.com/galaxy-io/filament"
)

const exportPollReportInterval = 30 * time.Second

// Each worker owns its progress. No captures or request data enter telemetry.
type jobProgress struct {
	observe           filament.SourceObserver
	resource          string
	data              filament.ExportJobProgress
	started, lastPoll time.Time
	readyReported     bool
}

func (c *Runtime) jobProgress(resource string, resumed bool) *jobProgress {
	return &jobProgress{observe: c.Observe, resource: resource, started: time.Now(),
		data: filament.ExportJobProgress{CorrelationID: rand.Text(), Resumed: resumed}}
}

func (p *jobProgress) report(kind filament.SourceProgressKind) {
	p.data.Elapsed = time.Since(p.started)
	p.observe.Report(filament.SourceProgress{Kind: kind, Resource: p.resource, ExportJob: p.data})
}

func (p *jobProgress) polled() {
	p.data.Polls++
	now := time.Now()
	if p.lastPoll.IsZero() || now.Sub(p.lastPoll) >= exportPollReportInterval {
		p.lastPoll = now
		p.report(filament.SourceProgressExportJobPolled)
	}
}

func (p *jobProgress) ready() {
	if !p.readyReported {
		p.readyReported = true
		p.report(filament.SourceProgressExportJobReady)
	}
}
