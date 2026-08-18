package iceberg

import (
	"bufio"
	"fmt"
	"os"
	"slices"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament"
)

// recordBuf accumulates one resource's batches for a pending commit. It holds
// them in memory until their byte count exceeds the configured limit, then spills
// every batch to an Arrow IPC file (LZ4) and streams the rest straight to disk.
// Row operations stay in memory (one byte per row) beside the batches. It is NOT
// safe for concurrent use; the sink serializes appends per stage.
type recordBuf struct {
	limitBytes int64
	bytes      int64 // Arrow buffer bytes appended
	count      int   // rows appended
	hasUpdate  bool
	hasDelete  bool
	policy     *filament.WritePolicy
	schema     *arrow.Schema

	// in-memory path: retained batches
	mem []arrow.RecordBatch

	// spill path — non-nil once the buffer has exceeded limitBytes
	file   *os.File
	bw     *bufio.Writer
	writer *ipc.Writer

	// ops holds each spilled or retained batch's operations, in batch order; nil
	// entries are all-insert batches.
	ops [][]filament.Operation
}

func newRecordBuf(limitBytes int64) *recordBuf {
	return &recordBuf{limitBytes: limitBytes}
}

func (rb *recordBuf) setPolicy(policy filament.WritePolicy) error {
	if rb.policy == nil {
		cp := policy
		cp.Keys = append([]string(nil), policy.Keys...)
		rb.policy = &cp
		return nil
	}
	if rb.policy.Capability.Mode != policy.Capability.Mode {
		return fmt.Errorf("mixed write policies %q and %q", rb.policy.Capability.Mode, policy.Capability.Mode)
	}
	if !slices.Equal(rb.policy.Keys, policy.Keys) {
		return fmt.Errorf("mixed primary keys %v and %v", rb.policy.Keys, policy.Keys)
	}
	return nil
}

func (rb *recordBuf) writeMode(fallback writeMode) writeMode {
	if rb.policy == nil {
		return fallback
	}
	switch rb.policy.Capability.Mode {
	case filament.WriteAppend:
		return writeModeAppend
	case filament.WriteReplace:
		return writeModeReplace
	case filament.WriteUpsert:
		return writeModeUpsert
	case filament.WriteDelete:
		return writeModeDelete
	case filament.WriteMerge:
		return writeModeMerge
	default:
		return fallback
	}
}

// append adds one batch, retaining it (the pipeline releases its reference after
// Apply) or spilling it. Every batch of a buffer must share one Arrow schema.
func (rb *recordBuf) append(rows arrow.RecordBatch, ops []filament.Operation, nbytes int64) error {
	if rb.schema == nil {
		rb.schema = rows.Schema()
	} else if !rb.schema.Equal(rows.Schema()) {
		return fmt.Errorf("batch schema differs from the buffer's")
	}
	rb.bytes += nbytes
	rb.count += int(rows.NumRows())
	for _, op := range ops {
		switch op {
		case filament.OpUpdate:
			rb.hasUpdate = true
		case filament.OpDelete:
			rb.hasDelete = true
		}
	}
	if ops != nil { // the batch's Ops belong to the pipeline; only Rows are ours to retain
		ops = append([]filament.Operation(nil), ops...)
	}
	rb.ops = append(rb.ops, ops)

	if rb.file == nil && rb.bytes > rb.limitBytes {
		if err := rb.spill(); err != nil {
			return err
		}
	}
	if rb.file != nil {
		return rb.writer.Write(rows)
	}
	rows.Retain()
	rb.mem = append(rb.mem, rows)
	return nil
}

func (rb *recordBuf) validate(mode writeMode) error {
	switch mode {
	case writeModeAppend:
		if rb.hasUpdate || rb.hasDelete {
			return fmt.Errorf("append mode does not support update/delete rows")
		}
	case writeModeReplace:
		if rb.hasDelete {
			return fmt.Errorf("replace mode does not support delete rows")
		}
	case writeModeUpsert:
		if rb.hasDelete {
			return fmt.Errorf("upsert mode does not support delete rows")
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

// spill moves the retained batches to a new IPC temp file and switches to disk
// mode.
func (rb *recordBuf) spill() error {
	f, err := os.CreateTemp("", "iceberg-stage-*.arrow")
	if err != nil {
		return fmt.Errorf("iceberg sink: spill: %w", err)
	}
	rb.file = f
	rb.bw = bufio.NewWriterSize(f, 1<<20)
	rb.writer = ipc.NewWriter(rb.bw, ipc.WithSchema(rb.schema), ipc.WithLZ4(), ipc.WithAllocator(memory.DefaultAllocator))
	for _, rows := range rb.mem {
		if err := rb.writer.Write(rows); err != nil {
			return err // rb.mem still owns every batch; close releases them
		}
	}
	for _, rows := range rb.mem {
		rows.Release()
	}
	rb.mem = nil
	return nil
}

// stream yields the buffered batches with their operations, in append order,
// reading a spilled file back lazily so the whole buffer is never resident at
// once. Each yielded batch belongs to fn for the duration of the call only. It
// may run more than once (a retried commit); nothing may be appended after it.
func (rb *recordBuf) stream(fn func(rows arrow.RecordBatch, ops []filament.Operation) error) error {
	if rb.file == nil {
		for i, rows := range rb.mem {
			if err := fn(rows, rb.ops[i]); err != nil {
				return err
			}
		}
		return nil
	}
	if rb.writer != nil { // first read: seal the stream
		if err := rb.writer.Close(); err != nil {
			return err
		}
		if err := rb.bw.Flush(); err != nil {
			return err
		}
		rb.writer = nil
	}
	if _, err := rb.file.Seek(0, 0); err != nil {
		return err
	}
	rdr, err := ipc.NewReader(bufio.NewReaderSize(rb.file, 1<<20), ipc.WithAllocator(memory.DefaultAllocator))
	if err != nil {
		return err
	}
	defer rdr.Release()
	i := 0
	for rdr.Next() {
		if err := fn(rdr.RecordBatch(), rb.ops[i]); err != nil {
			return err
		}
		i++
	}
	return rdr.Err()
}

// close releases retained batches and removes any temp file. Idempotent.
func (rb *recordBuf) close() {
	for _, rows := range rb.mem {
		rows.Release()
	}
	rb.mem = nil
	if rb.file != nil {
		_ = rb.file.Close()
		_ = os.Remove(rb.file.Name())
		rb.file = nil
	}
}
