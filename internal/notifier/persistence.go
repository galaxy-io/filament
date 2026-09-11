package notifier

import (
	"context"

	"github.com/galaxy-io/filament"
)

// Filter selects a pipeline's rules, including disabled ones.
type Filter struct {
	Tenant         filament.TenantID
	PipelineID     string
	IncludeDeleted bool
}

// Store is optional storage for pipeline notification rules.
// Each write transaction must check that the pipeline still exists and is not deleted.
// Updates and deletes require a matching version or return [filament.ErrVersionConflict].
type Store interface {
	// CreateNotifier adds a rule at version 1.
	CreateNotifier(ctx context.Context, n Notifier) (Notifier, error)
	// LoadNotifier includes deleted rules. Check DeletedAt before using one.
	LoadNotifier(ctx context.Context, tenant filament.TenantID, pipelineID, id string) (Notifier, error)
	// ListNotifiers returns matching rules ordered by ID.
	ListNotifiers(ctx context.Context, f Filter) ([]Notifier, error)
	// UpdateNotifier updates a rule and increments Version. Its ID, tenant,
	// pipeline, and type cannot change. Clean up old secrets only after success.
	UpdateNotifier(ctx context.Context, n Notifier) (Notifier, error)
	// DeleteNotifier marks a rule deleted and increments Version.
	// It returns the saved rule, including secret references for cleanup.
	DeleteNotifier(ctx context.Context, tenant filament.TenantID, pipelineID, id string, version int64) (Notifier, error)
}
