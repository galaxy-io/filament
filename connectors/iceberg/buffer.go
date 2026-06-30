package iceberg

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/galaxy-io/filament"
)

// recordBuf is a per-resource accumulator that starts in memory and spills to a
// temp file once its byte count exceeds the configured limit. It is NOT safe for
// concurrent use; the sink serializes appends per stage.
type recordBuf struct {
	limitBytes int64
	bytes      int64 // total payload bytes appended
	count      int   // records appended
	hasUpdate  bool
	hasDelete  bool
	policy     *ingestion.WritePolicy

	// in-memory path
	mem []recordEntry

	// spill path — non-nil once the buffer has exceeded limitBytes
	file   *os.File
	writer *bufio.Writer // line-delimited JSON (one object per line)
}

func newRecordBuf(limitBytes int64) *recordBuf {
	return &recordBuf{limitBytes: limitBytes}
}

type recordEntry struct {
	Op   ingestion.Operation   `json:"op"`
	Data json.RawMessage `json:"data"`
}

// append adds one JSON payload to the buffer, spilling to disk if needed.
func (rb *recordBuf) append(data json.RawMessage) error {
	return rb.appendRecord(data, ingestion.OpInsert)
}

func (rb *recordBuf) setPolicy(policy ingestion.WritePolicy) error {
	if rb.policy == nil {
		cp := policy
		cp.Keys = append([]string(nil), policy.Keys...)
		rb.policy = &cp
		return nil
	}
	if rb.policy.Capability.Mode != policy.Capability.Mode {
		return fmt.Errorf("mixed write policies %q and %q", rb.policy.Capability.Mode, policy.Capability.Mode)
	}
	if !sameStrings(rb.policy.Keys, policy.Keys) {
		return fmt.Errorf("mixed primary keys %v and %v", rb.policy.Keys, policy.Keys)
	}
	return nil
}

func (rb *recordBuf) writeMode(fallback writeMode) writeMode {
	if rb.policy == nil {
		return fallback
	}
	switch rb.policy.Capability.Mode {
	case ingestion.WriteAppend:
		return writeModeAppend
	case ingestion.WriteReplace:
		return writeModeReplace
	case ingestion.WriteUpsert:
		return writeModeUpsert
	case ingestion.WriteDelete:
		return writeModeDelete
	case ingestion.WriteMerge:
		return writeModeMerge
	default:
		return fallback
	}
}

func (rb *recordBuf) appendRecord(data json.RawMessage, op ingestion.Operation) error {
	rb.bytes += int64(len(data))
	rb.count++
	if op == ingestion.OpUpdate {
		rb.hasUpdate = true
	}
	if op == ingestion.OpDelete {
		rb.hasDelete = true
	}

	if rb.file == nil && rb.bytes > rb.limitBytes {
		if err := rb.spill(); err != nil {
			return err
		}
	}

	if rb.file != nil {
		return rb.writeEntry(recordEntry{Op: op, Data: data})
	}

	// Copy: the caller's Data slice may be reused after Write returns.
	cp := make(json.RawMessage, len(data))
	copy(cp, data)
	rb.mem = append(rb.mem, recordEntry{Op: op, Data: cp})
	return nil
}

func (rb *recordBuf) validate(mode writeMode) error {
	switch mode {
	case writeModeAppend:
		if rb.hasUpdate || rb.hasDelete {
			return fmt.Errorf("append mode does not support update/delete records")
		}
	case writeModeReplace:
		if rb.hasDelete {
			return fmt.Errorf("replace mode does not support delete records")
		}
	case writeModeUpsert:
		if rb.hasDelete {
			return fmt.Errorf("upsert mode does not support delete records")
		}
		if rb.policy == nil || len(rb.policy.Keys) == 0 {
			return fmt.Errorf("upsert mode requires primary key columns")
		}
	case writeModeDelete, writeModeMerge:
		if rb.policy == nil || len(rb.policy.Keys) == 0 {
			return fmt.Errorf("%s mode requires primary key columns", mode)
		}
	}
	return nil
}

// spill moves in-memory records to a new temp file and switches to disk mode.
func (rb *recordBuf) spill() error {
	f, err := os.CreateTemp("", "iceberg-stage-*.jsonl")
	if err != nil {
		return fmt.Errorf("iceberg sink: spill: %w", err)
	}
	rb.file = f
	rb.writer = bufio.NewWriterSize(f, 1<<20) // 1 MiB write buffer

	for _, entry := range rb.mem {
		if err := rb.writeEntry(entry); err != nil {
			return err
		}
	}
	rb.mem = nil
	return nil
}

func (rb *recordBuf) writeEntry(entry recordEntry) error {
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := rb.writer.Write(line); err != nil {
		return err
	}
	return rb.writer.WriteByte('\n')
}

// stream yields the buffered payloads in chunks of at most rowsPerChunk records,
// reading a spilled file back lazily so the whole buffer is never resident at
// once. The slice passed to fn is reused between calls — fn must consume it
// before returning. Must be called before close.
func (rb *recordBuf) stream(rowsPerChunk int, fn func([]json.RawMessage) error) error {
	return rb.streamEntries(rowsPerChunk, func(entries []recordEntry) error {
		recs := make([]json.RawMessage, len(entries))
		for i := range entries {
			recs[i] = entries[i].Data
		}
		return fn(recs)
	})
}

func (rb *recordBuf) streamEntries(rowsPerChunk int, fn func([]recordEntry) error) error {
	if rb.file == nil {
		for i := 0; i < len(rb.mem); i += rowsPerChunk {
			end := min(i+rowsPerChunk, len(rb.mem))
			if err := fn(rb.mem[i:end]); err != nil {
				return err
			}
		}
		return nil
	}

	if err := rb.writer.Flush(); err != nil {
		return err
	}
	if _, err := rb.file.Seek(0, 0); err != nil {
		return err
	}
	sc := bufio.NewScanner(rb.file)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	chunk := make([]recordEntry, 0, rowsPerChunk)
	for sc.Scan() {
		var entry recordEntry
		if err := json.Unmarshal(sc.Bytes(), &entry); err != nil {
			return err
		}
		if len(chunk) == rowsPerChunk {
			if err := fn(chunk); err != nil {
				return err
			}
			chunk = chunk[:0]
		}
		chunk = append(chunk, entry)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if len(chunk) > 0 {
		return fn(chunk)
	}
	return nil
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// close removes any temp file. Idempotent.
func (rb *recordBuf) close() {
	if rb.file != nil {
		_ = rb.file.Close()
		_ = os.Remove(rb.file.Name())
		rb.file = nil
	}
}
