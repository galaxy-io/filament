package integrity

import (
	"sync"
	"time"
)

// Checkpoint status values.
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusPartial   = "partial"
)

// ChunkID uniquely identifies a batch within a resource extraction.
type ChunkID struct {
	Resource string `json:"resource"`
	SeqNum   int    `json:"seq_num"`
}

// ChunkCommitment captures the dual-accumulator state for a single batch/chunk.
type ChunkCommitment struct {
	ID          ChunkID    `json:"id"`
	Read        Commitment `json:"read"`
	Write       Commitment `json:"write"`
	RecordCount int        `json:"record_count"`
	Verified    bool       `json:"verified"`
	Cursor      string     `json:"cursor,omitempty"`
}

// ResourceCheckpoint is the serializable per-resource checkpoint state.
type ResourceCheckpoint struct {
	Resource     string            `json:"resource"`
	Chunks       []ChunkCommitment `json:"chunks"`
	TotalRecords int64             `json:"total_records"`
	TotalBytes   int64             `json:"total_bytes"`
	// Watermark carries incremental-extraction state keyed by checkpoint_key,
	// e.g. {"gh_repos_since": "2026-04-14T00:00:00Z"}. Nil when the resource
	// does not use incremental extraction.
	Watermark map[string]string `json:"watermark,omitempty"`
	// Captures records the per-record fields the resource declared in its
	// `capture` block. Persisted so that, on resume, child resources can fan
	// out across already-completed parents without re-extracting them.
	Captures []map[string]string `json:"captures,omitempty"`
}

// RecordChunk appends a chunk commitment and updates running totals.
func (rc *ResourceCheckpoint) RecordChunk(cc ChunkCommitment) {
	rc.Chunks = append(rc.Chunks, cc)
	rc.TotalRecords += int64(cc.RecordCount)
}

// PipelineCheckpoint is the top-level serializable checkpoint for an extraction.
//
// Mutating methods are safe for concurrent use — the connector-side fan-out
// model produces simultaneous writes from multiple goroutines (per-resource
// extraction + per-parent child fan-out).
type PipelineCheckpoint struct {
	Connector    string                         `json:"connector"`
	StartedAt    time.Time                      `json:"started_at"`
	UpdatedAt    time.Time                      `json:"updated_at"`
	Resources    map[string]*ResourceCheckpoint `json:"resources"`
	Status       string                         `json:"status"` // running | completed | failed | partial
	FailedChunks []ChunkID                      `json:"failed_chunks,omitempty"`

	// mu serializes all mutating + reading access. Not exported because
	// callers should go through the methods below; JSON (un)marshal happens
	// without contention since persistence runs after extract completes.
	mu sync.Mutex `json:"-"`
}

// NewPipelineCheckpoint creates a checkpoint in "running" state.
func NewPipelineCheckpoint(connector string) *PipelineCheckpoint {
	now := time.Now().UTC()
	return &PipelineCheckpoint{
		Connector: connector,
		StartedAt: now,
		UpdatedAt: now,
		Resources: make(map[string]*ResourceCheckpoint),
		Status:    StatusRunning,
	}
}

// GetOrCreateResource returns the checkpoint for a resource, creating it if needed.
func (pc *PipelineCheckpoint) GetOrCreateResource(resource string) *ResourceCheckpoint {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.getOrCreateResourceLocked(resource)
}

// getOrCreateResourceLocked is the inner helper for callers that already hold pc.mu.
func (pc *PipelineCheckpoint) getOrCreateResourceLocked(resource string) *ResourceCheckpoint {
	rc, ok := pc.Resources[resource]
	if !ok {
		rc = &ResourceCheckpoint{Resource: resource}
		pc.Resources[resource] = rc
	}
	return rc
}

// MarkFailed records a chunk as failed and updates the checkpoint timestamp.
func (pc *PipelineCheckpoint) MarkFailed(id ChunkID) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.FailedChunks = append(pc.FailedChunks, id)
	pc.UpdatedAt = time.Now().UTC()
}

// RetryRequest is sent from the hot path to the retry coordinator
// when a chunk divergence is detected.
type RetryRequest struct {
	Chunk ChunkCommitment
	Lines [][]byte // pre-marshaled NDJSON lines for CRC re-verification
}

// RetryResult is produced by the retry coordinator after re-serialization.
type RetryResult struct {
	Chunk       ChunkID
	Success     bool
	Attempts    int
	Replacement []byte     // verified re-serialized bytes (nil if permanently failed)
	Commitment  Commitment // write-side commitment of the replacement
	Err         error
}

// IsResourceComplete returns true if a resource has at least one chunk
// and all its chunks are verified with no failures.
func (pc *PipelineCheckpoint) IsResourceComplete(resource string) bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	rc, ok := pc.Resources[resource]
	if !ok || len(rc.Chunks) == 0 {
		return false
	}
	for _, cc := range rc.Chunks {
		if !cc.Verified {
			return false
		}
	}
	for _, fid := range pc.FailedChunks {
		if fid.Resource == resource {
			return false
		}
	}
	return true
}

// SetWatermark records an incremental-extraction value (e.g. last observed
// updated_at) unconditionally. Prefer MergeWatermark for fan-out scenarios
// where multiple goroutines need a max-merge across competing values.
func (pc *PipelineCheckpoint) SetWatermark(resource, key, value string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	rc := pc.getOrCreateResourceLocked(resource)
	if rc.Watermark == nil {
		rc.Watermark = map[string]string{}
	}
	rc.Watermark[key] = value
	pc.UpdatedAt = time.Now().UTC()
}

// MergeWatermark atomically merges value into the resource's watermark. cmp
// returns >0 when a > b. The new value is kept iff cmp(value, existing) > 0
// (or no value exists yet). Safe for concurrent calls from fan-out goroutines.
func (pc *PipelineCheckpoint) MergeWatermark(resource, key, value string, cmp func(a, b string) int) {
	if value == "" {
		return
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()
	rc := pc.getOrCreateResourceLocked(resource)
	if rc.Watermark == nil {
		rc.Watermark = map[string]string{}
	}
	cur, ok := rc.Watermark[key]
	if !ok || cur == "" || cmp(value, cur) > 0 {
		rc.Watermark[key] = value
	}
	pc.UpdatedAt = time.Now().UTC()
}

// GetWatermark returns the stored watermark value for a resource + key, or ""
// if none.
func (pc *PipelineCheckpoint) GetWatermark(resource, key string) string {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	rc, ok := pc.Resources[resource]
	if !ok || rc.Watermark == nil {
		return ""
	}
	return rc.Watermark[key]
}

// AppendCaptures persists captured parent-record fields so that on resume,
// already-completed parents can still seed child fan-out without re-extracting.
// Safe for concurrent calls.
func (pc *PipelineCheckpoint) AppendCaptures(resource string, fields []map[string]string) {
	if len(fields) == 0 {
		return
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()
	rc := pc.getOrCreateResourceLocked(resource)
	rc.Captures = append(rc.Captures, fields...)
	pc.UpdatedAt = time.Now().UTC()
}

// GetCaptures returns the captured fields previously persisted for a resource.
// Returns nil if the resource has no captures recorded.
func (pc *PipelineCheckpoint) GetCaptures(resource string) []map[string]string {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	rc, ok := pc.Resources[resource]
	if !ok {
		return nil
	}
	return rc.Captures
}

// LastVerifiedCursor returns the cursor from the last verified chunk of a resource.
// Returns "" if no verified chunks exist or no cursor was recorded.
//
// Important: for child resources with parent fan-out, this returns ANY of the
// fan-out parents' last cursors interleaved — callers must not seed a single
// parent's pagination from a fan-out resource's resource-wide cursor. The
// httpapi connector handles this by skipping cursor resume for child resources.
func (pc *PipelineCheckpoint) LastVerifiedCursor(resource string) string {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	rc, ok := pc.Resources[resource]
	if !ok {
		return ""
	}
	for i := len(rc.Chunks) - 1; i >= 0; i-- {
		if rc.Chunks[i].Verified && rc.Chunks[i].Cursor != "" {
			return rc.Chunks[i].Cursor
		}
	}
	return ""
}

// MergeInto copies complete resources from this (previous) checkpoint into
// the current checkpoint, preserving the current checkpoint's data for
// any resource that was re-extracted.
func (pc *PipelineCheckpoint) MergeInto(current *PipelineCheckpoint) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	current.mu.Lock()
	defer current.mu.Unlock()
	for resource, rc := range pc.Resources {
		if _, reExtracted := current.Resources[resource]; reExtracted {
			continue
		}
		// inline IsResourceComplete check (we already hold pc's lock)
		complete := len(rc.Chunks) > 0
		for _, cc := range rc.Chunks {
			if !cc.Verified {
				complete = false
				break
			}
		}
		for _, fid := range pc.FailedChunks {
			if fid.Resource == resource {
				complete = false
				break
			}
		}
		if !complete {
			continue
		}
		current.Resources[resource] = rc
	}
}

// Complete sets the final status based on whether any chunks failed.
func (pc *PipelineCheckpoint) Complete() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.UpdatedAt = time.Now().UTC()
	if len(pc.FailedChunks) > 0 {
		pc.Status = StatusPartial
	} else {
		pc.Status = StatusCompleted
	}
}
