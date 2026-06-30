package pipeline

// EventType classifies extraction progress events.
type EventType int

const (
	EventResourceStart EventType = iota
	EventPageFetched
	EventResourceComplete
	EventResourceFailed
	EventFanOutStart
	EventBufferedBatch
	EventIntegrityVerified
	EventIntegrityFailed
	EventUploaded
	EventUploadFailed
	EventRateLimited
	EventWriterClosed
	EventChunkVerified
	EventChunkDivergence
	EventRetrySuccess
	EventRetryExhausted
	EventCheckpointSaved

	// EventWatermarkAdvanced fires once per resource extraction when the
	// incremental tracker first records an advance, with the new value in
	// Cursor (re-purposed for the watermark — Field carries the cursor
	// field name for clarity). Subsequent advances within the same
	// extraction are not re-emitted to keep the event stream sparse.
	EventWatermarkAdvanced
)

func (t EventType) String() string {
	switch t {
	case EventResourceStart:
		return "resource_start"
	case EventPageFetched:
		return "page_fetched"
	case EventResourceComplete:
		return "resource_complete"
	case EventResourceFailed:
		return "resource_failed"
	case EventFanOutStart:
		return "fan_out_start"
	case EventBufferedBatch:
		return "buffered_batch"
	case EventIntegrityVerified:
		return "integrity_verified"
	case EventIntegrityFailed:
		return "integrity_failed"
	case EventUploaded:
		return "uploaded"
	case EventUploadFailed:
		return "upload_failed"
	case EventRateLimited:
		return "rate_limited"
	case EventWriterClosed:
		return "writer_closed"
	case EventChunkVerified:
		return "chunk_verified"
	case EventChunkDivergence:
		return "chunk_divergence"
	case EventRetrySuccess:
		return "retry_success"
	case EventRetryExhausted:
		return "retry_exhausted"
	case EventCheckpointSaved:
		return "checkpoint_saved"
	case EventWatermarkAdvanced:
		return "watermark_advanced"
	default:
		return "unknown"
	}
}

// Event carries extraction progress information.
type Event struct {
	Resource        string
	Type            EventType
	Connector       string
	Records         int    // records in this page/batch
	TotalRecords    int    // cumulative for this resource
	Pages           int    // page count so far
	Bytes           int64  // bytes so far
	ParentsDone     int    // child resources: parents completed
	ParentsTotal    int    // child resources: total parent count
	Cursor          string // pagination cursor
	S3Key           string // for upload events
	CRCSum          string // for integrity events
	ChunkSeqNum     int    // for chunk-level events
	ReadCommitment  string // hex read-side accumulator sum
	WriteCommitment string // hex write-side accumulator sum
	CheckpointKey   string // S3 key of saved checkpoint
	MerkleRoot      string // hex merkle root
	Field           string // field name (e.g. cursor field for watermark events)
	Error           error
}
