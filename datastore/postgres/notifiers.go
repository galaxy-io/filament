package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
	"github.com/galaxy-io/filament/internal/notifier"
)

var _ notifier.Store = (*Store)(nil)

// CreateNotifier adds a rule at version 1.
func (s *Store) CreateNotifier(ctx context.Context, n notifier.Notifier) (notifier.Notifier, error) {
	kind, err := n.NotificationType.Label()
	if err != nil {
		return notifier.Notifier{}, err
	}
	data, err := marshalNotifier(n)
	if err != nil {
		return notifier.Notifier{}, err
	}
	return s.writeNotifier(ctx, n.Tenant, n.PipelineID, n.ID, func(q *sqlcgen.Queries) (*sqlcgen.Notifier, error) {
		return q.CreateNotifier(ctx, sqlcgen.CreateNotifierParams{
			NotifierID: n.ID, TenantID: string(n.Tenant), PipelineID: n.PipelineID,
			Name: n.Name, NotificationType: sqlcgen.NotificationType(kind), Enabled: n.Enabled,
			Events: data.events, Resources: data.resources, Config: data.config, SecretRefs: data.refs,
			CreatedByUserID: toText(n.CreatedByUserID), UpdatedByUserID: toText(n.UpdatedByUserID),
		})
	})
}

// LoadNotifier returns a rule, including deleted rules for secret cleanup.
func (s *Store) LoadNotifier(ctx context.Context, tenant filament.TenantID, pipelineID, id string) (notifier.Notifier, error) {
	row, err := s.q.GetNotifier(ctx, sqlcgen.GetNotifierParams{TenantID: string(tenant), PipelineID: pipelineID, NotifierID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return notifier.Notifier{}, filament.ErrNotFound
	}
	if err != nil {
		return notifier.Notifier{}, fmt.Errorf("datastore/postgres: load notifier: %w", err)
	}
	return notifierFromRow(row)
}

// ListNotifiers returns one pipeline's rules ordered by ID.
func (s *Store) ListNotifiers(ctx context.Context, f notifier.Filter) ([]notifier.Notifier, error) {
	rows, err := s.q.ListNotifiers(ctx, sqlcgen.ListNotifiersParams{
		TenantID: string(f.Tenant), PipelineID: f.PipelineID, IncludeDeleted: f.IncludeDeleted,
	})
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: list notifiers: %w", err)
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
		return notifier.Notifier{}, err
	}
	data, err := marshalNotifier(n)
	if err != nil {
		return notifier.Notifier{}, err
	}
	return s.writeNotifier(ctx, n.Tenant, n.PipelineID, n.ID, func(q *sqlcgen.Queries) (*sqlcgen.Notifier, error) {
		row, err := notifierForUpdate(ctx, q, n.Tenant, n.PipelineID, n.ID, n.Version)
		if err != nil {
			return nil, err
		}
		if string(row.NotificationType) != kind {
			return nil, fmt.Errorf("notifier notification type cannot change")
		}
		return q.UpdateNotifier(ctx, sqlcgen.UpdateNotifierParams{
			TenantID: string(n.Tenant), PipelineID: n.PipelineID, NotifierID: n.ID, ExpectedVersion: n.Version,
			Name: n.Name, Enabled: n.Enabled, Events: data.events, Resources: data.resources,
			Config: data.config, SecretRefs: data.refs, UpdatedByUserID: toText(n.UpdatedByUserID),
		})
	})
}

// DeleteNotifier marks a rule deleted and returns its metadata for secret cleanup.
func (s *Store) DeleteNotifier(ctx context.Context, tenant filament.TenantID, pipelineID, id string, version int64) (notifier.Notifier, error) {
	return s.writeNotifier(ctx, tenant, pipelineID, id, func(q *sqlcgen.Queries) (*sqlcgen.Notifier, error) {
		if _, err := notifierForUpdate(ctx, q, tenant, pipelineID, id, version); err != nil {
			return nil, err
		}
		return q.DeleteNotifier(ctx, sqlcgen.DeleteNotifierParams{
			TenantID: string(tenant), PipelineID: pipelineID, NotifierID: id, ExpectedVersion: version,
		})
	})
}

// Lock the parent before changing a rule so pipeline deletion cannot race it.
// Decode the returned row before committing; callers receive exactly what they wrote.
func (s *Store) writeNotifier(ctx context.Context, tenant filament.TenantID, pipelineID, id string, write func(*sqlcgen.Queries) (*sqlcgen.Notifier, error)) (notifier.Notifier, error) {
	if tenant == "" || pipelineID == "" || id == "" {
		return notifier.Notifier{}, fmt.Errorf("notifier tenant, pipeline id, and id are required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return notifier.Notifier{}, fmt.Errorf("datastore/postgres: begin notifier write: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if _, err := q.LockNotifierPipeline(ctx, sqlcgen.LockNotifierPipelineParams{TenantID: string(tenant), PipelineID: pipelineID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notifier.Notifier{}, filament.ErrNotFound
		}
		return notifier.Notifier{}, fmt.Errorf("datastore/postgres: lock notifier pipeline: %w", err)
	}
	row, err := write(q)
	if err != nil {
		return notifier.Notifier{}, fmt.Errorf("datastore/postgres: write notifier: %w", err)
	}
	n, err := notifierFromRow(row)
	if err != nil {
		return notifier.Notifier{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return notifier.Notifier{}, fmt.Errorf("datastore/postgres: commit notifier write: %w", err)
	}
	return n, nil
}

func notifierForUpdate(ctx context.Context, q *sqlcgen.Queries, tenant filament.TenantID, pipelineID, id string, version int64) (*sqlcgen.Notifier, error) {
	row, err := q.GetNotifier(ctx, sqlcgen.GetNotifierParams{TenantID: string(tenant), PipelineID: pipelineID, NotifierID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, filament.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if row.IsDeleted || row.Version != version {
		return nil, filament.ErrVersionConflict
	}
	return row, nil
}

func notifierFromRow(row *sqlcgen.Notifier) (notifier.Notifier, error) {
	kind, err := notifier.ParseNotificationType(string(row.NotificationType))
	if err != nil {
		return notifier.Notifier{}, err
	}
	n := notifier.Notifier{
		ID: row.ID, Tenant: filament.TenantID(row.TenantID), PipelineID: row.PipelineID,
		Name: row.Name, NotificationType: kind, Enabled: row.Enabled, Version: row.Version,
		CreatedAt: timestampMillis(row.CreatedAt), UpdatedAt: timestampMillis(row.UpdatedAt), DeletedAt: timestampMillis(row.DeletedAt),
		CreatedByUserID: row.CreatedByUserID.String, UpdatedByUserID: row.UpdatedByUserID.String, DeletedByUserID: row.DeletedByUserID.String,
	}
	for _, field := range []struct {
		name   string
		data   []byte
		target any
	}{
		{"events", row.Events, &n.Events},
		{"resources", row.Resources, &n.Resources},
		{"config", row.Config, &n.Config},
		{"secret_refs", row.SecretRefs, &n.SecretRefs},
	} {
		if err := json.Unmarshal(field.data, field.target); err != nil {
			return notifier.Notifier{}, fmt.Errorf("datastore/postgres: decode notifier %s: %w", field.name, err)
		}
	}
	return n, nil
}

type notifierJSON struct{ events, resources, config, refs []byte }

func marshalNotifier(n notifier.Notifier) (notifierJSON, error) {
	if n.Config == nil {
		n.Config = map[string]any{}
	}
	if n.SecretRefs == nil {
		n.SecretRefs = map[string]string{}
	}
	var data notifierJSON
	for _, field := range []struct {
		name   string
		value  any
		target *[]byte
	}{
		{"events", append([]string{}, n.Events...), &data.events},
		{"resources", append([]string{}, n.Resources...), &data.resources},
		{"config", n.Config, &data.config},
		{"secret_refs", n.SecretRefs, &data.refs},
	} {
		raw, err := json.Marshal(field.value)
		if err != nil {
			return notifierJSON{}, fmt.Errorf("datastore/postgres: encode notifier %s: %w", field.name, err)
		}
		*field.target = raw
	}
	return data, nil
}
