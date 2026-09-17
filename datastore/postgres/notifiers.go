package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
	"github.com/galaxy-io/filament/internal/notifier"
)

// CreateNotifier adds a rule.
func (s *Store) CreateNotifier(ctx context.Context, n *ingestionv1.Notifier) (*ingestionv1.Notifier, error) {
	if n == nil {
		return nil, fmt.Errorf("create notifier: notifier is required")
	}
	kind, err := notifierTypeToRow(n.GetNotificationType())
	if err != nil {
		return nil, fmt.Errorf("create notifier %q: %w", n.GetId(), err)
	}
	data, err := marshalNotifier(n)
	if err != nil {
		return nil, fmt.Errorf("create notifier %q: %w", n.GetId(), err)
	}
	saved, err := s.writeNotifier(ctx, filament.TenantID(n.GetTenantId()), n.GetPipelineId(), n.GetId(), func(q *sqlcgen.Queries) (*sqlcgen.Notifier, error) {
		return q.CreateNotifier(ctx, sqlcgen.CreateNotifierParams{
			NotifierID: n.GetId(), TenantID: n.GetTenantId(), PipelineID: n.GetPipelineId(),
			Name: n.GetName(), NotificationType: kind, IsEnabled: n.GetIsEnabled(),
			Events: data.events, Resources: data.resources, Config: data.config, SecretRefs: data.refs,
			CreatedByUserID: toText(n.GetCreatedByUserId()), UpdatedByUserID: toText(n.GetUpdatedByUserId()),
		})
	})
	if err != nil {
		return nil, fmt.Errorf("create notifier %q: %w", n.GetId(), err)
	}
	return saved, nil
}

// LoadNotifier returns a rule, including deleted rules for secret cleanup.
func (s *Store) LoadNotifier(ctx context.Context, tenant filament.TenantID, pipelineID, id string) (*ingestionv1.Notifier, error) {
	row, err := s.q.GetNotifier(ctx, sqlcgen.GetNotifierParams{TenantID: string(tenant), PipelineID: pipelineID, NotifierID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("load notifier %q: %w", id, filament.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: load notifier %q: %w", id, err)
	}
	return notifierFromRow(row)
}

// ListNotifiers returns one pipeline's rules ordered by ID.
func (s *Store) ListNotifiers(ctx context.Context, tenant filament.TenantID, pipelineID string, includeDeleted bool) ([]*ingestionv1.Notifier, error) {
	rows, err := s.q.ListNotifiers(ctx, sqlcgen.ListNotifiersParams{
		TenantID: string(tenant), PipelineID: pipelineID, IncludeDeleted: includeDeleted,
	})
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: list notifiers: %w", err)
	}
	out := make([]*ingestionv1.Notifier, 0, len(rows))
	for _, row := range rows {
		n, err := notifierFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

// UpdateNotifier replaces a live rule's settings.
func (s *Store) UpdateNotifier(ctx context.Context, n *ingestionv1.Notifier) (*ingestionv1.Notifier, error) {
	if n == nil {
		return nil, fmt.Errorf("update notifier: notifier is required")
	}
	kind, err := notifierTypeToRow(n.GetNotificationType())
	if err != nil {
		return nil, fmt.Errorf("update notifier %q: %w", n.GetId(), err)
	}
	data, err := marshalNotifier(n)
	if err != nil {
		return nil, fmt.Errorf("update notifier %q: %w", n.GetId(), err)
	}
	tenant := filament.TenantID(n.GetTenantId())
	saved, err := s.writeNotifier(ctx, tenant, n.GetPipelineId(), n.GetId(), func(q *sqlcgen.Queries) (*sqlcgen.Notifier, error) {
		row, err := liveNotifier(ctx, q, tenant, n.GetPipelineId(), n.GetId())
		if err != nil {
			return nil, err
		}
		if row.NotificationType != kind {
			return nil, fmt.Errorf("notifier notification type cannot change")
		}
		return q.UpdateNotifier(ctx, sqlcgen.UpdateNotifierParams{
			TenantID: n.GetTenantId(), PipelineID: n.GetPipelineId(), NotifierID: n.GetId(),
			Name: n.GetName(), IsEnabled: n.GetIsEnabled(), Events: data.events, Resources: data.resources,
			Config: data.config, SecretRefs: data.refs, UpdatedByUserID: toText(n.GetUpdatedByUserId()),
		})
	})
	if err != nil {
		return nil, fmt.Errorf("update notifier %q: %w", n.GetId(), err)
	}
	return saved, nil
}

// DeleteNotifier marks a live rule deleted and returns it for secret cleanup.
func (s *Store) DeleteNotifier(ctx context.Context, tenant filament.TenantID, pipelineID, id string) (*ingestionv1.Notifier, error) {
	saved, err := s.writeNotifier(ctx, tenant, pipelineID, id, func(q *sqlcgen.Queries) (*sqlcgen.Notifier, error) {
		if _, err := liveNotifier(ctx, q, tenant, pipelineID, id); err != nil {
			return nil, err
		}
		return q.DeleteNotifier(ctx, sqlcgen.DeleteNotifierParams{TenantID: string(tenant), PipelineID: pipelineID, NotifierID: id})
	})
	if err != nil {
		return nil, fmt.Errorf("delete notifier %q: %w", id, err)
	}
	return saved, nil
}

// Lock the parent before changing a rule so pipeline deletion cannot race it.
// Decode the returned row before committing; callers receive exactly what they wrote.
func (s *Store) writeNotifier(ctx context.Context, tenant filament.TenantID, pipelineID, id string, write func(*sqlcgen.Queries) (*sqlcgen.Notifier, error)) (*ingestionv1.Notifier, error) {
	if tenant == "" || pipelineID == "" || id == "" {
		return nil, fmt.Errorf("notifier tenant, pipeline id, and id are required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: begin notifier write: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if _, err := q.LockNotifierPipeline(ctx, sqlcgen.LockNotifierPipelineParams{TenantID: string(tenant), PipelineID: pipelineID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("pipeline %q: %w", pipelineID, filament.ErrNotFound)
		}
		return nil, fmt.Errorf("datastore/postgres: lock notifier pipeline: %w", err)
	}
	row, err := write(q)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("notifier %q: %w", id, filament.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: write notifier: %w", err)
	}
	n, err := notifierFromRow(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("datastore/postgres: commit notifier write: %w", err)
	}
	return n, nil
}

func liveNotifier(ctx context.Context, q *sqlcgen.Queries, tenant filament.TenantID, pipelineID, id string) (*sqlcgen.Notifier, error) {
	row, err := q.GetNotifier(ctx, sqlcgen.GetNotifierParams{TenantID: string(tenant), PipelineID: pipelineID, NotifierID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, filament.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if row.IsDeleted {
		return nil, filament.ErrNotFound
	}
	return row, nil
}

func notifierFromRow(row *sqlcgen.Notifier) (*ingestionv1.Notifier, error) {
	kind, err := notifierTypeFromRow(row.NotificationType)
	if err != nil {
		return nil, err
	}
	n := &ingestionv1.Notifier{
		Id: row.ID, TenantId: row.TenantID, PipelineId: row.PipelineID,
		Name: row.Name, NotificationType: kind, IsEnabled: row.IsEnabled,
		CreatedAt: timestampMillis(row.CreatedAt), UpdatedAt: timestampMillis(row.UpdatedAt), DeletedAt: timestampMillis(row.DeletedAt),
		CreatedByUserId: row.CreatedByUserID.String, UpdatedByUserId: row.UpdatedByUserID.String, DeletedByUserId: row.DeletedByUserID.String,
	}
	var names []string
	if err := json.Unmarshal(row.Events, &names); err != nil {
		return nil, fmt.Errorf("datastore/postgres: unmarshal notifier %q events: %w", row.ID, err)
	}
	if n.Events, err = notifierEventsFromRow(names); err != nil {
		return nil, fmt.Errorf("datastore/postgres: notifier %q: %w", row.ID, err)
	}
	if err := json.Unmarshal(row.Resources, &n.Resources); err != nil {
		return nil, fmt.Errorf("datastore/postgres: unmarshal notifier %q resources: %w", row.ID, err)
	}
	var config map[string]any
	if err := json.Unmarshal(row.Config, &config); err != nil {
		return nil, fmt.Errorf("datastore/postgres: unmarshal notifier %q config: %w", row.ID, err)
	}
	if n.Config, err = structpb.NewStruct(config); err != nil {
		return nil, fmt.Errorf("datastore/postgres: notifier %q config: %w", row.ID, err)
	}
	if err := json.Unmarshal(row.SecretRefs, &n.SecretRefs); err != nil {
		return nil, fmt.Errorf("datastore/postgres: unmarshal notifier %q secret_refs: %w", row.ID, err)
	}
	return n, nil
}

func notifierEventsFromRow(names []string) ([]ingestionv1.NotifierEvent, error) {
	events := make([]ingestionv1.NotifierEvent, 0, len(names))
	for _, name := range names {
		event, err := notifier.ParseEventName(name)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func notifierEventsToRow(events []ingestionv1.NotifierEvent) ([]string, error) {
	names := make([]string, 0, len(events))
	for _, event := range events {
		name, err := notifier.EventName(event)
		if err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

func notifierTypeToRow(kind ingestionv1.NotificationType) (sqlcgen.NotificationType, error) {
	switch kind {
	case ingestionv1.NotificationType_NOTIFICATION_TYPE_WEBHOOK:
		return sqlcgen.NotificationTypeWebhook, nil
	default:
		return "", fmt.Errorf("invalid notification type %d", kind)
	}
}

func notifierTypeFromRow(kind sqlcgen.NotificationType) (ingestionv1.NotificationType, error) {
	switch kind {
	case sqlcgen.NotificationTypeWebhook:
		return ingestionv1.NotificationType_NOTIFICATION_TYPE_WEBHOOK, nil
	default:
		return ingestionv1.NotificationType_NOTIFICATION_TYPE_UNSPECIFIED, fmt.Errorf("invalid notification type %q", kind)
	}
}

type notifierJSON struct{ events, resources, config, refs []byte }

func marshalNotifier(n *ingestionv1.Notifier) (notifierJSON, error) {
	events, err := notifierEventsToRow(n.GetEvents())
	if err != nil {
		return notifierJSON{}, err
	}
	resources := n.GetResources()
	if resources == nil {
		resources = []string{}
	}
	config := n.GetConfig().AsMap()
	if config == nil {
		config = map[string]any{}
	}
	refs := n.GetSecretRefs()
	if refs == nil {
		refs = map[string]string{}
	}
	var data notifierJSON
	data.events, err = json.Marshal(events)
	if err != nil {
		return notifierJSON{}, fmt.Errorf("datastore/postgres: marshal notifier events: %w", err)
	}
	data.resources, err = json.Marshal(resources)
	if err != nil {
		return notifierJSON{}, fmt.Errorf("datastore/postgres: marshal notifier resources: %w", err)
	}
	data.config, err = json.Marshal(config)
	if err != nil {
		return notifierJSON{}, fmt.Errorf("datastore/postgres: marshal notifier config: %w", err)
	}
	data.refs, err = json.Marshal(refs)
	if err != nil {
		return notifierJSON{}, fmt.Errorf("datastore/postgres: marshal notifier secret_refs: %w", err)
	}
	return data, nil
}
