package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite/sqlcgen"
	"github.com/galaxy-io/filament/internal/notifier"
)

var _ notifier.Store = (*Store)(nil)

// CreateNotifier adds a rule at version 1.
func (s *Store) CreateNotifier(ctx context.Context, n notifier.Notifier) (notifier.Notifier, error) {
	kind, err := n.NotificationType.Label()
	if err != nil {
		return notifier.Notifier{}, fmt.Errorf("create notifier %q: %w", n.ID, err)
	}
	data, err := marshalNotifier(n)
	if err != nil {
		return notifier.Notifier{}, fmt.Errorf("create notifier %q: %w", n.ID, err)
	}
	saved, err := s.writeNotifier(ctx, n.Tenant, n.PipelineID, n.ID, func(q *sqlcgen.Queries) (*sqlcgen.Notifier, error) {
		now := nowMillis()
		return q.CreateNotifier(ctx, sqlcgen.CreateNotifierParams{
			NotifierID: n.ID, TenantID: string(n.Tenant), PipelineID: n.PipelineID,
			CreatedAt: now, UpdatedAt: now,
			Name: n.Name, NotificationType: kind, IsEnabled: boolInt(n.IsEnabled),
			Events: string(data.events), Resources: string(data.resources), Config: string(data.config), SecretRefs: string(data.refs),
			CreatedByUserID: sql.NullString{String: n.CreatedByUserID, Valid: n.CreatedByUserID != ""}, UpdatedByUserID: sql.NullString{String: n.UpdatedByUserID, Valid: n.UpdatedByUserID != ""},
		})
	})
	if err != nil {
		return notifier.Notifier{}, fmt.Errorf("create notifier %q: %w", n.ID, err)
	}
	return saved, nil
}

// LoadNotifier returns a rule, including deleted rules for secret cleanup.
func (s *Store) LoadNotifier(ctx context.Context, tenant filament.TenantID, pipelineID, id string) (notifier.Notifier, error) {
	row, err := s.q.GetNotifier(ctx, sqlcgen.GetNotifierParams{TenantID: string(tenant), PipelineID: pipelineID, NotifierID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return notifier.Notifier{}, fmt.Errorf("load notifier %q: %w", id, filament.ErrNotFound)
	}
	if err != nil {
		return notifier.Notifier{}, fmt.Errorf("datastore/sqlite: load notifier %q: %w", id, err)
	}
	return notifierFromRow(row)
}

// ListNotifiers returns one pipeline's rules ordered by ID.
func (s *Store) ListNotifiers(ctx context.Context, f notifier.Filter) ([]notifier.Notifier, error) {
	rows, err := s.q.ListNotifiers(ctx, sqlcgen.ListNotifiersParams{
		TenantID: string(f.Tenant), PipelineID: f.PipelineID, IncludeDeleted: f.IncludeDeleted,
	})
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: list notifiers: %w", err)
	}
	out := make([]notifier.Notifier, 0, len(rows))
	for _, row := range rows {
		n, err := notifierFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

// UpdateNotifier replaces settings when the stored version matches.
func (s *Store) UpdateNotifier(ctx context.Context, n notifier.Notifier) (notifier.Notifier, error) {
	kind, err := n.NotificationType.Label()
	if err != nil {
		return notifier.Notifier{}, fmt.Errorf("update notifier %q at version %d: %w", n.ID, n.Version, err)
	}
	data, err := marshalNotifier(n)
	if err != nil {
		return notifier.Notifier{}, fmt.Errorf("update notifier %q at version %d: %w", n.ID, n.Version, err)
	}
	saved, err := s.writeNotifier(ctx, n.Tenant, n.PipelineID, n.ID, func(q *sqlcgen.Queries) (*sqlcgen.Notifier, error) {
		row, err := notifierForUpdate(ctx, q, n.Tenant, n.PipelineID, n.ID, n.Version)
		if err != nil {
			return nil, err
		}
		if row.NotificationType != kind {
			return nil, fmt.Errorf("notifier notification type cannot change")
		}
		return q.UpdateNotifier(ctx, sqlcgen.UpdateNotifierParams{
			TenantID: string(n.Tenant), PipelineID: n.PipelineID, NotifierID: n.ID, ExpectedVersion: n.Version,
			UpdatedAt: nowMillis(),
			Name:      n.Name, IsEnabled: boolInt(n.IsEnabled), Events: string(data.events), Resources: string(data.resources),
			Config: string(data.config), SecretRefs: string(data.refs), UpdatedByUserID: sql.NullString{String: n.UpdatedByUserID, Valid: n.UpdatedByUserID != ""},
		})
	})
	if err != nil {
		return notifier.Notifier{}, fmt.Errorf("update notifier %q at version %d: %w", n.ID, n.Version, err)
	}
	return saved, nil
}

// DeleteNotifier marks a rule deleted and returns its metadata for secret cleanup.
func (s *Store) DeleteNotifier(ctx context.Context, tenant filament.TenantID, pipelineID, id string, version int64) (notifier.Notifier, error) {
	saved, err := s.writeNotifier(ctx, tenant, pipelineID, id, func(q *sqlcgen.Queries) (*sqlcgen.Notifier, error) {
		if _, err := notifierForUpdate(ctx, q, tenant, pipelineID, id, version); err != nil {
			return nil, err
		}
		now := nowMillis()
		return q.DeleteNotifier(ctx, sqlcgen.DeleteNotifierParams{
			TenantID: string(tenant), PipelineID: pipelineID, NotifierID: id, ExpectedVersion: version,
			DeletedAt: sql.NullInt64{Int64: now, Valid: true}, UpdatedAt: now,
		})
	})
	if err != nil {
		return notifier.Notifier{}, fmt.Errorf("delete notifier %q at version %d: %w", id, version, err)
	}
	return saved, nil
}

// Lock the parent before changing a rule so pipeline deletion cannot race it.
// Decode the returned row before committing; callers receive exactly what they wrote.
func (s *Store) writeNotifier(ctx context.Context, tenant filament.TenantID, pipelineID, id string, write func(*sqlcgen.Queries) (*sqlcgen.Notifier, error)) (notifier.Notifier, error) {
	if tenant == "" || pipelineID == "" || id == "" {
		return notifier.Notifier{}, fmt.Errorf("notifier tenant, pipeline id, and id are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return notifier.Notifier{}, fmt.Errorf("datastore/sqlite: begin notifier write: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	if _, err := q.LockNotifierPipeline(ctx, sqlcgen.LockNotifierPipelineParams{TenantID: string(tenant), PipelineID: pipelineID}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notifier.Notifier{}, fmt.Errorf("pipeline %q: %w", pipelineID, filament.ErrNotFound)
		}
		return notifier.Notifier{}, fmt.Errorf("datastore/sqlite: lock notifier pipeline: %w", err)
	}
	row, err := write(q)
	if err != nil {
		return notifier.Notifier{}, fmt.Errorf("datastore/sqlite: write notifier: %w", err)
	}
	n, err := notifierFromRow(row)
	if err != nil {
		return notifier.Notifier{}, err
	}
	if err := tx.Commit(); err != nil {
		return notifier.Notifier{}, fmt.Errorf("datastore/sqlite: commit notifier write: %w", err)
	}
	return n, nil
}

func notifierForUpdate(ctx context.Context, q *sqlcgen.Queries, tenant filament.TenantID, pipelineID, id string, version int64) (*sqlcgen.Notifier, error) {
	row, err := q.GetNotifier(ctx, sqlcgen.GetNotifierParams{TenantID: string(tenant), PipelineID: pipelineID, NotifierID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, filament.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if row.IsDeleted != 0 || row.Version != version {
		return nil, filament.ErrVersionConflict
	}
	return row, nil
}

func notifierFromRow(row *sqlcgen.Notifier) (notifier.Notifier, error) {
	kind, err := notifier.ParseNotificationType(row.NotificationType)
	if err != nil {
		return notifier.Notifier{}, err
	}
	n := notifier.Notifier{
		ID: row.ID, Tenant: filament.TenantID(row.TenantID), PipelineID: row.PipelineID,
		Name: row.Name, NotificationType: kind, IsEnabled: row.IsEnabled != 0, Version: row.Version,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt.Int64,
		CreatedByUserID: row.CreatedByUserID.String, UpdatedByUserID: row.UpdatedByUserID.String, DeletedByUserID: row.DeletedByUserID.String,
	}
	if err := json.Unmarshal([]byte(row.Events), &n.Events); err != nil {
		return notifier.Notifier{}, fmt.Errorf("datastore/sqlite: unmarshal notifier %q events: %w", row.ID, err)
	}
	if err := json.Unmarshal([]byte(row.Resources), &n.Resources); err != nil {
		return notifier.Notifier{}, fmt.Errorf("datastore/sqlite: unmarshal notifier %q resources: %w", row.ID, err)
	}
	if err := json.Unmarshal([]byte(row.Config), &n.Config); err != nil {
		return notifier.Notifier{}, fmt.Errorf("datastore/sqlite: unmarshal notifier %q config: %w", row.ID, err)
	}
	if err := json.Unmarshal([]byte(row.SecretRefs), &n.SecretRefs); err != nil {
		return notifier.Notifier{}, fmt.Errorf("datastore/sqlite: unmarshal notifier %q secret_refs: %w", row.ID, err)
	}
	return n, nil
}

type notifierJSON struct{ events, resources, config, refs []byte }

func marshalNotifier(n notifier.Notifier) (notifierJSON, error) {
	if n.Events == nil {
		n.Events = []string{}
	}
	if n.Resources == nil {
		n.Resources = []string{}
	}
	if n.Config == nil {
		n.Config = map[string]any{}
	}
	if n.SecretRefs == nil {
		n.SecretRefs = map[string]string{}
	}
	var data notifierJSON
	var err error
	data.events, err = json.Marshal(n.Events)
	if err != nil {
		return notifierJSON{}, fmt.Errorf("datastore/sqlite: marshal notifier events: %w", err)
	}
	data.resources, err = json.Marshal(n.Resources)
	if err != nil {
		return notifierJSON{}, fmt.Errorf("datastore/sqlite: marshal notifier resources: %w", err)
	}
	data.config, err = json.Marshal(n.Config)
	if err != nil {
		return notifierJSON{}, fmt.Errorf("datastore/sqlite: marshal notifier config: %w", err)
	}
	data.refs, err = json.Marshal(n.SecretRefs)
	if err != nil {
		return notifierJSON{}, fmt.Errorf("datastore/sqlite: marshal notifier secret_refs: %w", err)
	}
	return data, nil
}
