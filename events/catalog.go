package events

import (
	"time"

	"github.com/galaxy-io/filament"
)

// The catalog: every fact filament emits, with its typed payload. Wire
// spellings match the pre-typed-events subject grammar so subjects, stream
// configuration, and hook rules survive the cutover unchanged.

type (
	// RunRequestedEvent marks a run accepted and queued for dispatch.
	RunRequestedEvent struct{}
	// RunStartedEvent marks a worker beginning extraction.
	RunStartedEvent struct{}
	// RunCompletedEvent carries the run's final counters.
	RunCompletedEvent struct {
		Records int64 `json:"records"`
		Bytes   int64 `json:"bytes"`
	}
	// RunFailedEvent carries the error that ended the run.
	RunFailedEvent struct {
		Error string `json:"error"`
	}
	// RunPartialEvent marks a run that landed some resources but not all.
	RunPartialEvent struct {
		Error string `json:"error"`
	}
	// HeartbeatEvent is the worker liveness and resource usage signal: the
	// pod's cumulative cgroup CPU time and its current/peak working set.
	HeartbeatEvent struct {
		CPUSeconds      float64 `json:"cpuSeconds,omitempty"`
		MemoryBytes     int64   `json:"memoryBytes,omitempty"`
		MemoryPeakBytes int64   `json:"memoryPeakBytes,omitempty"`
	}

	// ResourceStartedEvent marks extraction beginning for one resource.
	ResourceStartedEvent struct{}
	// PageFetchedEvent counts one fetched page of a resource.
	PageFetchedEvent struct {
		Records int64  `json:"records"`
		Bytes   int64  `json:"bytes"`
		URI     string `json:"uri,omitempty"`
	}
	// ResourceCompletedEvent carries a resource's final counters.
	ResourceCompletedEvent struct {
		Records int64 `json:"records"`
		Bytes   int64 `json:"bytes"`
	}
	// ResourceFailedEvent carries the error that ended a resource.
	ResourceFailedEvent struct {
		Error string `json:"error"`
	}

	// BatchBufferedEvent counts records staged into an in-memory batch.
	BatchBufferedEvent struct {
		Records int64 `json:"records"`
		Bytes   int64 `json:"bytes"`
	}
	// BatchWrittenEvent marks one batch durably applied to the sink.
	BatchWrittenEvent struct {
		Records    int64                    `json:"records"`
		Bytes      int64                    `json:"bytes"`
		URI        string                   `json:"uri,omitempty"`
		CRC        uint32                   `json:"crc,omitempty"`
		Checkpoint *filament.CheckpointData `json:"checkpoint,omitempty"`
	}
	// IntegrityVerifiedEvent marks a batch's CRC re-checked after write.
	IntegrityVerifiedEvent struct {
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
)

// The event kinds, one per payload type above; each value is the capability
// to emit or subscribe to that kind.
var (
	RunRequested = define[RunRequestedEvent]("run.requested")
	RunStarted   = define[RunStartedEvent]("run.started")
	RunCompleted = define[RunCompletedEvent]("run.completed")
	RunFailed    = define[RunFailedEvent]("run.failed")
	RunPartial   = define[RunPartialEvent]("run.partial")
	Heartbeat    = define[HeartbeatEvent]("run.heartbeat")

	ResourceStarted   = define[ResourceStartedEvent]("resource.started")
	PageFetched       = define[PageFetchedEvent]("resource.page_fetched")
	ResourceCompleted = define[ResourceCompletedEvent]("resource.completed")
	ResourceFailed    = define[ResourceFailedEvent]("resource.failed")

	BatchBuffered     = define[BatchBufferedEvent]("batch.buffered")
	BatchWritten      = define[BatchWrittenEvent]("batch.written")
	IntegrityVerified = define[IntegrityVerifiedEvent]("batch.integrity_verified")
	ChunkDivergence   = define[ChunkDivergenceEvent]("batch.chunk_divergence")

	WatermarkAdvanced = define[WatermarkAdvancedEvent]("cursor.watermark_advanced")
	CheckpointSaved   = define[CheckpointSavedEvent]("cursor.checkpoint_saved")

	RateLimited    = define[RateLimitedEvent]("pressure.rate_limited")
	RetryExhausted = define[RetryExhaustedEvent]("pressure.retry_exhausted")

	ScheduleFired = define[ScheduleFiredEvent]("schedule.fired")
)
