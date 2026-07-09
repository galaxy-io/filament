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
	RunRequested = Define[RunRequestedEvent]("run.requested")
	RunStarted   = Define[RunStartedEvent]("run.started")
	RunCompleted = Define[RunCompletedEvent]("run.completed")
	RunFailed    = Define[RunFailedEvent]("run.failed")
	RunPartial   = Define[RunPartialEvent]("run.partial")
	Heartbeat    = Define[HeartbeatEvent]("run.heartbeat")

	ResourceStarted   = Define[ResourceStartedEvent]("resource.started")
	PageFetched       = Define[PageFetchedEvent]("resource.page_fetched")
	ResourceCompleted = Define[ResourceCompletedEvent]("resource.completed")
	ResourceFailed    = Define[ResourceFailedEvent]("resource.failed")

	BatchBuffered     = Define[BatchBufferedEvent]("batch.buffered")
	BatchWritten      = Define[BatchWrittenEvent]("batch.written")
	IntegrityVerified = Define[IntegrityVerifiedEvent]("batch.integrity_verified")
	ChunkDivergence   = Define[ChunkDivergenceEvent]("batch.chunk_divergence")

	WatermarkAdvanced = Define[WatermarkAdvancedEvent]("cursor.watermark_advanced")
	CheckpointSaved   = Define[CheckpointSavedEvent]("cursor.checkpoint_saved")

	RateLimited    = Define[RateLimitedEvent]("pressure.rate_limited")
	RetryExhausted = Define[RetryExhaustedEvent]("pressure.retry_exhausted")

	ScheduleFired = Define[ScheduleFiredEvent]("schedule.fired")
)
