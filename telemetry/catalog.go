package telemetry

import (
	"context"
	"time"
)

// Attribute keys.
const (
	attrRunStatus      = "filament.run.status"
	attrSourceType     = "filament.source.type"
	attrSinkType       = "filament.sink.type"
	attrPipelineID     = "filament.pipeline.id"
	attrResourceName   = "filament.resource.name"
	attrWebhookOutcome = "filament.webhook.outcome"
	attrErrorType      = "error.type"
)

var (
	runBuckets   = []float64{1, 5, 15, 30, 60, 120, 300, 600, 1800, 3600, 7200, 14400, 28800, 86400}
	queueBuckets = []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600, 1800, 3600}
)

const (
	statusCompleted = "completed"
	statusFailed    = "failed"
)

// Part is a part of Filament that records counters of its own.
type Part int

// The parts that record counters known before anything happens.
const (
	Orchestrator Part = iota
	Dispatcher
	Reaper
	Tracker
	Scheduler
	WebhookRetention
)

// Every metric is declared here, once.
var (
	runsSubmitted  = counter("filament.run.submissions", "{run}", "Runs accepted and requested.")
	runsDispatched = counter("filament.dispatches", "{run}", "Runs handed to a worker.")
	dispatchFailed = counter("filament.dispatch.failures", "{failure}", "Runs that could not be handed to a worker.", attrErrorType)
	runsReaped     = counter("filament.run.reaps", "{run}", "Stale runs the reaper failed.")
	runQueue       = histogram("filament.run.queue.duration", "s", "How long a run waited from submission to starting on a worker.", queueBuckets, attrPipelineID)

	runsTotal   = counter("filament.runs", "{run}", "Runs that reached a terminal status.", attrRunStatus, attrSourceType, attrSinkType, attrPipelineID, attrErrorType)
	runDuration = histogram("filament.run.duration", "s", "How long a run took from start to its terminal status.", runBuckets, attrRunStatus, attrPipelineID, attrErrorType)
	lastSuccess = gauge("filament.pipeline.last_success.time", "s", "When a pipeline's last run completed, as a Unix timestamp.", attrPipelineID)
	lastFailure = gauge("filament.pipeline.last_failure.time", "s", "When a pipeline's last run failed, as a Unix timestamp.", attrPipelineID)

	recordsRead           = counter("filament.source.records", "{record}", "Records read from sources.", attrPipelineID, attrResourceName)
	recordsWritten        = counter("filament.sink.records", "{record}", "Records written to sinks.", attrPipelineID, attrResourceName)
	recordsRejected       = counter("filament.records.rejected", "{record}", "Records read from sources but not written to sinks.", attrPipelineID, attrResourceName, attrErrorType)
	bytesWritten          = counter("filament.sink.io", "By", "Bytes written to sinks.", attrPipelineID, attrResourceName)
	checkpointFlushFailed = counter("filament.checkpoint.failures", "{failure}", "Cursor writes that failed and were only logged.")

	scheduleFires           = counter("filament.schedule.fires", "{run}", "Scheduled runs the scheduler requested.")
	scheduleOverlapSkips    = counter("filament.schedule.skips", "{run}", "Scheduled runs skipped because the last one was still running.")
	scheduleTickFailures    = counter("filament.schedule.tick.failures", "{failure}", "Scheduler ticks that failed.")
	scheduleSubmitFailures  = counter("filament.schedule.submit.failures", "{failure}", "Scheduled runs that could not be requested.")
	scheduleReconcileFailed = counter("filament.schedule.reconcile.failures", "{failure}", "Schedule reconciliations that failed.")

	webhookRequests   = counter("filament.webhook.requests", "{request}", "Webhook deliveries received, by how they were answered.", attrWebhookOutcome)
	webhookRejected   = counter("filament.webhook.rejections", "{delivery}", "Stored deliveries a source could not turn into rows.", attrResourceName)
	webhookPurged     = counter("filament.webhook.inbox.purges", "{delivery}", "Deliveries removed from the inbox after their rows were written.")
	webhookExpired    = counter("filament.webhook.inbox.expirations", "{delivery}", "Deliveries removed from the inbox unread, past retention.")
	webhookOldest     = gauge("filament.webhook.inbox.oldest_wait.duration", "s", "How long the oldest delivery still in the inbox has waited.")
	webhookInboxCount = gauge("filament.webhook.inbox.messages", "{message}", "Deliveries held in the inbox.")
	webhookInboxBytes = gauge("filament.webhook.inbox.size", "By", "Bytes held in the inbox.")

	workerCPU    = counter("filament.worker.cpu.time", "s", "CPU time used by workers.", attrPipelineID)
	workerMemory = gauge("filament.worker.memory.usage", "By", "Memory a worker is using.", attrPipelineID)
)

// Set records Filament's metrics. Build one with New. A nil Set records
// nothing, so callers never check for one.
type Set struct {
	runsSubmitted, runsDispatched, dispatchFailed, runsReaped Counter
	runQueue                                                  Histogram

	runsTotal                Counter
	runDuration              Histogram
	lastSuccess, lastFailure Gauge

	recordsRead, recordsWritten, recordsRejected, bytesWritten, checkpointFlushFailed Counter

	scheduleFires, scheduleOverlapSkips, scheduleTickFailures Counter
	scheduleSubmitFailures, scheduleReconcileFailed           Counter

	webhookRequests, webhookRejected, webhookPurged, webhookExpired Counter
	webhookOldest, webhookInboxCount, webhookInboxBytes             Gauge

	workerCPU    Counter
	workerMemory Gauge
}

// New binds every metric to an instrument of b.
func New(b Backend) *Set {
	return &Set{
		runsSubmitted:  b.Counter(runsSubmitted),
		runsDispatched: b.Counter(runsDispatched),
		dispatchFailed: b.Counter(dispatchFailed),
		runsReaped:     b.Counter(runsReaped),
		runQueue:       b.Histogram(runQueue),

		runsTotal:   b.Counter(runsTotal),
		runDuration: b.Histogram(runDuration),
		lastSuccess: b.Gauge(lastSuccess),
		lastFailure: b.Gauge(lastFailure),

		recordsRead:           b.Counter(recordsRead),
		recordsWritten:        b.Counter(recordsWritten),
		recordsRejected:       b.Counter(recordsRejected),
		bytesWritten:          b.Counter(bytesWritten),
		checkpointFlushFailed: b.Counter(checkpointFlushFailed),

		scheduleFires:           b.Counter(scheduleFires),
		scheduleOverlapSkips:    b.Counter(scheduleOverlapSkips),
		scheduleTickFailures:    b.Counter(scheduleTickFailures),
		scheduleSubmitFailures:  b.Counter(scheduleSubmitFailures),
		scheduleReconcileFailed: b.Counter(scheduleReconcileFailed),

		webhookRequests:   b.Counter(webhookRequests),
		webhookRejected:   b.Counter(webhookRejected),
		webhookPurged:     b.Counter(webhookPurged),
		webhookExpired:    b.Counter(webhookExpired),
		webhookOldest:     b.Gauge(webhookOldest),
		webhookInboxCount: b.Gauge(webhookInboxCount),
		webhookInboxBytes: b.Gauge(webhookInboxBytes),

		workerCPU:    b.Counter(workerCPU),
		workerMemory: b.Gauge(workerMemory),
	}
}

// Started records a zero for the counters part records whose attributes are
// none or one of a few known values. A counter is first reported when it is
// first added to, and a query for its increase cannot see that first value,
// so one that starts at zero has every increase counted.
func (s *Set) Started(ctx context.Context, part Part) {
	if s == nil {
		return
	}
	var counters []Counter
	switch part {
	case Orchestrator:
		counters = []Counter{s.runsSubmitted}
	case Dispatcher:
		counters = []Counter{s.runsDispatched}
		for _, errorType := range []string{ErrorTypeForbidden, ErrorTypeInvalidSpec, ErrorTypeTimeout, ErrorTypeAPI, ErrorTypeOther} {
			s.dispatchFailed.Add(ctx, 0, errorType)
		}
	case Reaper:
		counters = []Counter{s.runsReaped}
	case Tracker:
		counters = []Counter{s.checkpointFlushFailed}
	case Scheduler:
		counters = []Counter{s.scheduleFires, s.scheduleOverlapSkips, s.scheduleTickFailures, s.scheduleSubmitFailures, s.scheduleReconcileFailed}
	case WebhookRetention:
		counters = []Counter{s.webhookPurged, s.webhookExpired}
	}
	for _, c := range counters {
		c.Add(ctx, 0)
	}
}

// RunSubmitted counts a run that was accepted and requested.
func (s *Set) RunSubmitted(ctx context.Context) {
	if s != nil {
		s.runsSubmitted.Add(ctx, 1)
	}
}

// RunDispatched counts a run handed to a worker.
func (s *Set) RunDispatched(ctx context.Context) {
	if s != nil {
		s.runsDispatched.Add(ctx, 1)
	}
}

// DispatchFailed counts a run that could not be handed to a worker, by the
// error.type value of why.
func (s *Set) DispatchFailed(ctx context.Context, errorType string) {
	if s != nil {
		s.dispatchFailed.Add(ctx, 1, errorType)
	}
}

// RunReaped counts a stale run the reaper failed.
func (s *Set) RunReaped(ctx context.Context) {
	if s != nil {
		s.runsReaped.Add(ctx, 1)
	}
}

// RunStarted records how long a run waited from submission to starting on a
// worker. A wait that is not positive is unknown and left out.
func (s *Set) RunStarted(ctx context.Context, pipeline string, waited time.Duration) {
	if s != nil && waited > 0 {
		s.runQueue.Observe(ctx, waited.Seconds(), pipeline)
	}
}

// Run is a run that reached a terminal status.
type Run struct {
	Pipeline string
	Source   string
	Sink     string
	Status   string
	// ErrorType is the error.type value of why a failed run failed. It is
	// recorded only with the failed status, as ErrorTypeOther when empty.
	ErrorType string
	// Took is how long the run took, zero when its start is unknown.
	Took time.Duration
	// EndedAt is when the run reached its terminal status.
	EndedAt time.Time
}

// RunEnded records a run's terminal status, its duration when known and, for
// a run that completed or failed, when its pipeline last did.
func (s *Set) RunEnded(ctx context.Context, r Run) {
	if s == nil {
		return
	}
	var errorType string
	if r.Status == statusFailed {
		errorType = r.ErrorType
		if errorType == "" {
			errorType = ErrorTypeOther
		}
	}
	s.runsTotal.Add(ctx, 1, r.Status, r.Source, r.Sink, r.Pipeline, errorType)
	if r.Took > 0 {
		s.runDuration.Observe(ctx, r.Took.Seconds(), r.Status, r.Pipeline, errorType)
	}
	if r.EndedAt.IsZero() {
		return
	}
	switch r.Status {
	case statusCompleted:
		s.lastSuccess.Set(ctx, float64(r.EndedAt.Unix()), r.Pipeline)
	case statusFailed:
		s.lastFailure.Set(ctx, float64(r.EndedAt.Unix()), r.Pipeline)
	}
}

// BatchWritten adds the records and bytes a batch wrote for one resource. The
// control plane counts a bounded run's records, read, written and rejected,
// from its facts. A continuous run's worker counts its own.
func (s *Set) BatchWritten(ctx context.Context, pipeline, resource string, records, bytes int64) {
	if s == nil {
		return
	}
	s.recordsWritten.Add(ctx, float64(records), pipeline, resource)
	s.bytesWritten.Add(ctx, float64(bytes), pipeline, resource)
}

// RecordsRead adds the records sources handed to a run for one resource.
func (s *Set) RecordsRead(ctx context.Context, pipeline, resource string, records int64) {
	if s != nil && records > 0 {
		s.recordsRead.Add(ctx, float64(records), pipeline, resource)
	}
}

// RecordsRejected adds records read for one resource that were never written,
// by the error.type value of why. An empty errorType means no failure caused it.
func (s *Set) RecordsRejected(ctx context.Context, pipeline, resource, errorType string, records int64) {
	if s != nil && records > 0 {
		s.recordsRejected.Add(ctx, float64(records), pipeline, resource, errorType)
	}
}

// CheckpointFlushFailed counts a cursor write that failed and was only logged.
func (s *Set) CheckpointFlushFailed(ctx context.Context) {
	if s != nil {
		s.checkpointFlushFailed.Add(ctx, 1)
	}
}

// ScheduleFired counts a scheduled run the scheduler requested.
func (s *Set) ScheduleFired(ctx context.Context) {
	if s != nil {
		s.scheduleFires.Add(ctx, 1)
	}
}

// ScheduleOverlapSkipped counts a scheduled run skipped because the last one was still running.
func (s *Set) ScheduleOverlapSkipped(ctx context.Context) {
	if s != nil {
		s.scheduleOverlapSkips.Add(ctx, 1)
	}
}

// ScheduleTickFailed counts a scheduler tick that failed.
func (s *Set) ScheduleTickFailed(ctx context.Context) {
	if s != nil {
		s.scheduleTickFailures.Add(ctx, 1)
	}
}

// ScheduleSubmitFailed counts a scheduled run that could not be requested.
func (s *Set) ScheduleSubmitFailed(ctx context.Context) {
	if s != nil {
		s.scheduleSubmitFailures.Add(ctx, 1)
	}
}

// ScheduleReconcileFailed counts a schedule reconciliation that failed.
func (s *Set) ScheduleReconcileFailed(ctx context.Context) {
	if s != nil {
		s.scheduleReconcileFailed.Add(ctx, 1)
	}
}

// WebhookRequest counts a webhook delivery by how it was answered.
func (s *Set) WebhookRequest(ctx context.Context, outcome string) {
	if s != nil {
		s.webhookRequests.Add(ctx, 1, outcome)
	}
}

// WebhookDeliveryRejected counts a stored delivery a source could not turn into rows.
func (s *Set) WebhookDeliveryRejected(ctx context.Context, resource string) {
	if s != nil {
		s.webhookRejected.Add(ctx, 1, resource)
	}
}

// WebhookSwept records one retention sweep of the inbox: what it removed and
// how long the oldest delivery left has waited.
func (s *Set) WebhookSwept(ctx context.Context, purged, expired uint64, oldestWaiting time.Duration) {
	if s == nil {
		return
	}
	s.webhookPurged.Add(ctx, float64(purged))
	s.webhookExpired.Add(ctx, float64(expired))
	s.webhookOldest.Set(ctx, oldestWaiting.Seconds())
}

// WebhookInboxUsage sets what the inbox holds.
func (s *Set) WebhookInboxUsage(ctx context.Context, messages, bytes uint64) {
	if s == nil {
		return
	}
	s.webhookInboxCount.Set(ctx, float64(messages))
	s.webhookInboxBytes.Set(ctx, float64(bytes))
}

// WorkerUsage records one sample of a worker: the CPU seconds it used since
// the last sample and the memory it holds.
func (s *Set) WorkerUsage(ctx context.Context, pipeline string, cpuSeconds float64, memoryBytes int64) {
	if s == nil {
		return
	}
	if cpuSeconds > 0 {
		s.workerCPU.Add(ctx, cpuSeconds, pipeline)
	}
	s.workerMemory.Set(ctx, float64(memoryBytes), pipeline)
}
