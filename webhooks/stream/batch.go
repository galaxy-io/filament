package stream

import (
	"errors"
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
)

const (
	defaultBatchMaxRecords = 1000
	// MaxBatchRecords is the largest batch_max_records a connection may set.
	MaxBatchRecords      = 10_000
	defaultBatchMaxBytes = 4 << 20
	maxBatchBytes        = 64 << 20
	defaultBatchMaxWait  = time.Second
)

// Batch decides when a read closes: the first limit reached wins.
type Batch struct {
	MaxRecords int
	MaxBytes   int64
	MaxWait    time.Duration
}

// DefaultBatch is the batch a connection gets when it sets nothing.
func DefaultBatch() Batch {
	return Batch{MaxRecords: defaultBatchMaxRecords, MaxBytes: defaultBatchMaxBytes, MaxWait: defaultBatchMaxWait}
}

// BatchFields are the connection settings ParseBatch reads. Webhook-driven
// connectors add them to their config schema.
func BatchFields() []filament.ConfigField {
	return []filament.ConfigField{
		{
			Name: "batch_max_records", Type: filament.FieldInt, Default: defaultBatchMaxRecords,
			Scope: filament.ScopeConnection, Help: "Commit after this many webhook deliveries.",
		},
		{
			Name: "batch_max_bytes", Type: filament.FieldInt, Default: defaultBatchMaxBytes,
			Scope: filament.ScopeConnection, Help: "Commit after this many payload bytes.",
		},
		{
			Name: "batch_max_wait", Type: filament.FieldDuration, Default: "1s",
			Scope: filament.ScopeConnection, Help: "Commit a partial batch after this much time.",
		},
	}
}

// ParseBatch reads and validates the BatchFields settings.
func ParseBatch(cfg filament.Config) (Batch, error) {
	b := DefaultBatch()
	if cfg.Has("batch_max_records") {
		b.MaxRecords = cfg.Int("batch_max_records")
	}
	if cfg.Has("batch_max_bytes") {
		b.MaxBytes = int64(cfg.Int("batch_max_bytes"))
	}
	if cfg.Has("batch_max_wait") {
		b.MaxWait = cfg.Duration("batch_max_wait")
	}
	switch {
	case b.MaxRecords < 1 || b.MaxRecords > MaxBatchRecords:
		return Batch{}, fmt.Errorf("batch_max_records must be between 1 and %d", MaxBatchRecords)
	case b.MaxBytes < 1 || b.MaxBytes > maxBatchBytes:
		return Batch{}, fmt.Errorf("batch_max_bytes must be between 1 and %d", maxBatchBytes)
	case b.MaxWait < 10*time.Millisecond || b.MaxWait > time.Minute:
		return Batch{}, errors.New("batch_max_wait must be between 10ms and 1m")
	}
	return b, nil
}
