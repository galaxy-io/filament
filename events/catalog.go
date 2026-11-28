package events

import (
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/schema"
)

// The catalog: every fact filament emits, with its typed payload. Wire
// spellings match the pre-typed-events subject grammar so subjects, stream
// configuration, and hook rules survive the cutover unchanged.

type (
	// SchemaDetectedEvent reports observed drift before migration or certification.
	SchemaDetectedEvent schema.Detection
	// SchemaBlockedEvent reports drift that requires intervention. It carries no rows.
	SchemaBlockedEvent struct {
		Reasons []string `json:"reasons"`
	}

	// ExportJobCreatedEvent reports successful creation and validated captures.
	ExportJobCreatedEvent filament.ExportJobProgress
	// ExportJobPolledEvent samples completed status checks; Polls is cumulative.
	ExportJobPolledEvent filament.ExportJobProgress
	// ExportJobReadyEvent reports validated artifact availability, not sink commit.
	ExportJobReadyEvent filament.ExportJobProgress

	// StreamAttemptEndedEvent is an observational worker-lifetime event, not a
	// terminal run event: the same continuous run may pause or retry.
	StreamAttemptEndedEvent struct {
		Error string `json:"error,omitempty"`
	}

	// RunRequestedEvent marks a run accepted and queued for dispatch.
	RunRequestedEvent struct{}
	// RunStartedEvent marks a worker beginning extraction.
	RunStartedEvent struct{}
	// RunCompletedEvent carries the run's final counters.
	RunCompletedEvent struct {
		Records int64 `json:"records"`
		Bytes   int64 `json:"bytes"`
		// FailedResources lists resources that failed while the run isolated
		// resource failures. The sink committed every other resource.
		FailedResources []string `json:"failedResources,omitempty"`
	}
	// RunFailedEvent carries the error that ended the run. Records and Bytes
	// are the engine's counters when the engine reports the failure, and zero
	// when another process does.
	RunFailedEvent struct {
		Error   string `json:"error"`
		Records int64  `json:"records,omitempty"`
		Bytes   int64  `json:"bytes,omitempty"`
	}
	// RunPartialEvent marks a run that landed some resources but not all.
	RunPartialEvent struct {
		Error   string `json:"error"`
		Records int64  `json:"records,omitempty"`
		Bytes   int64  `json:"bytes,omitempty"`
	}
	// RunCanceledEvent marks a run stopped by a cancellation request. Records
	// and Bytes are set as for RunFailedEvent.
	RunCanceledEvent struct {
		Records int64 `json:"records,omitempty"`
		Bytes   int64 `json:"bytes,omitempty"`
		// Commanded marks the outcome of a cancel command, published by the
		// tracker after it stored the status. It never moves the status.
		Commanded bool `json:"commanded,omitempty"`
	}
	// RunPausedEvent marks a run suspended for a later continuation. Committed
	// reports whether the sink reached a boundary that can promote checkpoints.
	RunPausedEvent struct {
		Committed bool `json:"committed"`
		// Commanded marks the outcome of a pause command, published by the
		// tracker after it stored the status. It never moves the status.
		Commanded bool `json:"commanded,omitempty"`
	}
	// RunPauseRequestedEvent asks the worker owning a running run to stop
	// extraction, drain accepted records, and acknowledge with RunPaused.
	RunPauseRequestedEvent struct{}
	// RunCancelRequestedEvent asks the worker owning a running run to stop and
	// abort its sink before acknowledging with RunCanceled.
	RunCancelRequestedEvent struct{}
	// RunPauseCommandedEvent is a pause accepted for a run that is not
	// executing. The tracker folds it into RunPaused if the run is still
	// requested when it is folded, then publishes RunPaused.
	RunPauseCommandedEvent struct{}
	// RunResumeCommandedEvent is a resume accepted for a paused or partial
	// run. The tracker folds it into RunRequested with a fresh attempt, then
	// publishes RunRequested so the run is dispatched.
	RunResumeCommandedEvent struct{}
	// RunCancelCommandedEvent is a cancel accepted for a run that is not
	// executing. The tracker folds it into RunCanceled if the run is still
	// requested, paused or partial, then publishes RunCanceled.
	RunCancelCommandedEvent struct{}
	// HeartbeatEvent is the worker liveness and resource usage signal: the
	// pod's cumulative cgroup CPU time and its current/peak working set.
	HeartbeatEvent struct {
		CPUSeconds      float64 `json:"cpuSeconds,omitempty"`
		MemoryBytes     int64   `json:"memoryBytes,omitempty"`
		MemoryPeakBytes int64   `json:"memoryPeakBytes,omitempty"`
	}

	// ResourceStartedEvent marks preparation beginning for one resource.
	ResourceStartedEvent struct{}
	// PageFetchedEvent counts one fetched page of a resource.
	PageFetchedEvent struct {
		Records int64  `json:"records"`
		Bytes   int64  `json:"bytes"`
		URI     string `json:"uri,omitempty"`
	}
	// FanOutStartedEvent marks a parent-driven resource beginning concurrent
	// child extraction.
	FanOutStartedEvent struct {
		ParentsTotal int64 `json:"parentsTotal"`
	}
	// ResourceCompletedEvent carries a resource's final counters.
	ResourceCompletedEvent struct {
		Records int64 `json:"records"`
		Bytes   int64 `json:"bytes"`
	}
	// ResourceFailedEvent carries the error that ended a resource.
	ResourceFailedEvent struct {
		Error string `json:"error"`
		// Discarded means the resource's commit-gated writes were aborted
		// while the run went on, so its tentative cursor and tallies are void.
		Discarded bool `json:"discarded,omitempty"`
	}
	// ResourceQuarantinedEvent reports that a failed resource was stopped
	// while the rest of the run continues. The terminal resource.failed
	// follows when the run ends.
	ResourceQuarantinedEvent struct {
		Error string `json:"error"`
	}

	// BatchBufferedEvent counts records staged into an in-memory batch.
	BatchBufferedEvent struct {
		Records int64 `json:"records"`
		Bytes   int64 `json:"bytes"`
	}
	// BatchTransformedEvent records one batch through the transform stage: its
	// size after the plan and the time the plan took.
	BatchTransformedEvent struct {
		Records  int64         `json:"records"`
		Bytes    int64         `json:"bytes"`
		Duration time.Duration `json:"duration"`
	}
	// FunctionInvokedEvent records one batch through an alchemy function.
	FunctionInvokedEvent struct {
		Function     string        `json:"function"`
		Version      int64         `json:"version"`
		Step         string        `json:"step"`
		InvocationID string        `json:"invocationId"`
		Records      int64         `json:"records"`
		Attempts     int32         `json:"attempts"`
		Replayed     bool          `json:"replayed,omitempty"`
		Duration     time.Duration `json:"duration"`
	}
	// FunctionFailedEvent records a failed alchemy call; InvocationID is empty if nothing ran.
	FunctionFailedEvent struct {
		Function     string        `json:"function"`
		Version      int64         `json:"version"`
		Step         string        `json:"step"`
		InvocationID string        `json:"invocationId,omitempty"`
		Code         string        `json:"code"`
		Error        string        `json:"error"`
		Duration     time.Duration `json:"duration"`
	}
	// BatchWrittenEvent marks one batch durably applied to the sink.
	BatchWrittenEvent struct {
		Records          int64                     `json:"records"`
		Bytes            int64                     `json:"bytes"`
		URI              string                    `json:"uri,omitempty"`
		CRC              uint32                    `json:"crc,omitempty"`
		Checkpoint       *filament.CheckpointData  `json:"checkpoint,omitempty"`
		CheckpointPolicy filament.CheckpointPolicy `json:"checkpointPolicy,omitempty"`
	}
	// IntegrityVerifiedEvent marks a batch's CRC re-checked after write.
	IntegrityVerifiedEvent struct {
		CRC uint32 `json:"crc,omitempty"`
	}
	// EncodedIntegrityVerifiedEvent records a sink's verified checksum over the
	// exact serialized bytes handed to its transport.
	EncodedIntegrityVerifiedEvent struct {
		CRC uint32 `json:"crc,omitempty"`
	}
	// ChunkDivergenceEvent reports a CRC mismatch between staged and written data.
	ChunkDivergenceEvent struct {
		CRC   uint32 `json:"crc,omitempty"`
		Error string `json:"error,omitempty"`
	}

	// WatermarkAdvancedEvent marks the incremental cursor moving forward.
	WatermarkAdvancedEvent struct {
		Checkpoint *filament.CheckpointData `json:"checkpoint,omitempty"`
	}
	// CheckpointSavedEvent marks a resume point persisted.
	CheckpointSavedEvent struct {
		Checkpoint *filament.CheckpointData `json:"checkpoint,omitempty"`
	}

	// RateLimitedEvent reports source back-pressure and the advised wait.
	RateLimitedEvent struct {
		RetryAfter time.Duration `json:"retryAfter"`
	}
	// RetryExhaustedEvent reports a retry budget spent without success.
	RetryExhaustedEvent struct {
		Error string `json:"error,omitempty"`
	}

	// ScheduleFiredEvent marks a schedule triggering a run request.
	ScheduleFiredEvent struct{}

	// NotifierAttemptedEvent records an observed notification operation.
	NotifierAttemptedEvent struct {
		NotifierID            string `json:"notifier_id"`
		NotificationType      string `json:"notification_type"`
		PipelineID            string `json:"pipeline_id"`
		PipelineVersionID     string `json:"pipeline_version_id,omitempty"`
		DeliveryID            string `json:"delivery_id"`
		AttemptID             string `json:"attempt_id"`
		TriggerType           string `json:"trigger_type"`
		TriggerStreamSequence uint64 `json:"trigger_stream_sequence,string"`
		Outcome               string `json:"outcome"`
		RequestAttempted      bool   `json:"request_attempted"`
		Retryable             bool   `json:"retryable"`
		StatusCode            int    `json:"status_code,omitempty"`
		DurationMs            int64  `json:"duration_ms"`
		ErrorCode             string `json:"error_code,omitempty"`
	}
)

// The event kinds, one per payload type above; each value is the capability
// to emit or subscribe to that kind.
var (
	ExportJobCreated = define[ExportJobCreatedEvent]("export.job_created")
	ExportJobPolled  = define[ExportJobPolledEvent]("export.job_polled")
	ExportJobReady   = define[ExportJobReadyEvent]("export.job_ready")

	StreamAttemptEnded = define[StreamAttemptEndedEvent]("stream.attempt_ended")
	RunRequested       = define[RunRequestedEvent]("run.requested")
	RunStarted         = define[RunStartedEvent]("run.started")
	RunCompleted       = define[RunCompletedEvent]("run.completed")
	RunFailed          = define[RunFailedEvent]("run.failed")
	RunPartial         = define[RunPartialEvent]("run.partial")
	RunCanceled        = define[RunCanceledEvent]("run.canceled")
	RunPaused          = define[RunPausedEvent]("run.paused")
	RunPauseRequested  = define[RunPauseRequestedEvent]("run.pause_requested")
	RunCancelRequested = define[RunCancelRequestedEvent]("run.cancel_requested")
	RunPauseCommanded  = define[RunPauseCommandedEvent]("run.pause_commanded")
	RunResumeCommanded = define[RunResumeCommandedEvent]("run.resume_commanded")
	RunCancelCommanded = define[RunCancelCommandedEvent]("run.cancel_commanded")
	Heartbeat          = define[HeartbeatEvent]("run.heartbeat")

	ResourceStarted     = define[ResourceStartedEvent]("resource.started")
	PageFetched         = define[PageFetchedEvent]("resource.page_fetched")
	FanOutStarted       = define[FanOutStartedEvent]("resource.fan_out_started")
	ResourceCompleted   = define[ResourceCompletedEvent]("resource.completed")
	SchemaDetected      = define[SchemaDetectedEvent]("schema.detected")
	SchemaBlocked       = define[SchemaBlockedEvent]("schema.blocked")
	ResourceFailed      = define[ResourceFailedEvent]("resource.failed")
	ResourceQuarantined = define[ResourceQuarantinedEvent]("resource.quarantined")

	BatchBuffered            = define[BatchBufferedEvent]("batch.buffered")
	BatchTransformed         = define[BatchTransformedEvent]("batch.transformed")
	FunctionInvoked          = define[FunctionInvokedEvent]("batch.function_invoked")
	FunctionFailed           = define[FunctionFailedEvent]("batch.function_failed")
	BatchWritten             = define[BatchWrittenEvent]("batch.written")
	IntegrityVerified        = define[IntegrityVerifiedEvent]("batch.integrity_verified")
	EncodedIntegrityVerified = define[EncodedIntegrityVerifiedEvent]("batch.encoded_integrity_verified")
	ChunkDivergence          = define[ChunkDivergenceEvent]("batch.chunk_divergence")

	WatermarkAdvanced = define[WatermarkAdvancedEvent]("cursor.watermark_advanced")
	CheckpointSaved   = define[CheckpointSavedEvent]("cursor.checkpoint_saved")

	RateLimited    = define[RateLimitedEvent]("pressure.rate_limited")
	RetryExhausted = define[RetryExhaustedEvent]("pressure.retry_exhausted")

	ScheduleFired = define[ScheduleFiredEvent]("schedule.fired")

	NotifierAttempted = define[NotifierAttemptedEvent]("notifier.attempted")
)
