package events

import (
	"time"

	"github.com/galaxy-io/filament"
)

// The catalog: every fact filament emits, with its typed payload. Wire
// spellings match the pre-typed-events subject grammar so subjects, stream
// configuration, and hook rules survive the cutover unchanged.

type (
	RunRequestedEvent struct{}
	RunStartedEvent   struct{}
	RunCompletedEvent struct {
		Records int64 `json:"records"`
		Bytes   int64 `json:"bytes"`
	}
	RunFailedEvent struct {
		Error string `json:"error"`
	}
	RunPartialEvent struct {
		Error string `json:"error"`
	}
	HeartbeatEvent struct{}

	ResourceStartedEvent struct{}
	PageFetchedEvent     struct {
		Records int64  `json:"records"`
		Bytes   int64  `json:"bytes"`
		URI     string `json:"uri,omitempty"`
	}
	ResourceCompletedEvent struct {
		Records int64 `json:"records"`
		Bytes   int64 `json:"bytes"`
	}
	ResourceFailedEvent struct {
		Error string `json:"error"`
	}

	BatchBufferedEvent struct {
		Records int64 `json:"records"`
		Bytes   int64 `json:"bytes"`
	}
	BatchWrittenEvent struct {
		Records    int64                     `json:"records"`
		Bytes      int64                     `json:"bytes"`
		CRC        uint32                    `json:"crc,omitempty"`
		Checkpoint *ingestion.CheckpointData `json:"checkpoint,omitempty"`
	}
	IntegrityVerifiedEvent struct {
		CRC uint32 `json:"crc,omitempty"`
	}
	ChunkDivergenceEvent struct {
		Error string `json:"error,omitempty"`
	}

	WatermarkAdvancedEvent struct {
		Checkpoint *ingestion.CheckpointData `json:"checkpoint,omitempty"`
	}
	CheckpointSavedEvent struct {
		Checkpoint *ingestion.CheckpointData `json:"checkpoint,omitempty"`
	}

	RateLimitedEvent struct {
		RetryAfter time.Duration `json:"retryAfter"`
	}
	RetryExhaustedEvent struct {
		Error string `json:"error,omitempty"`
	}

	ScheduleFiredEvent struct{}
)

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
