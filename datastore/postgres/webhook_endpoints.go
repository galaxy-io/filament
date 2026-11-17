package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
	"github.com/galaxy-io/filament/webhooks"
	"github.com/galaxy-io/filament/webhooks/inbox"
)

var (
	_ webhooks.EndpointLoader = (*Store)(nil)
	_ webhooks.EndpointStore  = (*Store)(nil)
)

// WebhookReleases reports, per endpoint, how far every live consumer of its
// source has certified progress in the given inbox. One query covers every
// endpoint, so retention cost does not grow with delivery volume.
func (s *Store) WebhookReleases(ctx context.Context, stream string, incarnation time.Time) ([]inbox.Release, error) {
	rows, err := s.q.ListWebhookReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("list webhook releases: %w", err)
	}
	releases := make([]inbox.Release, 0, len(rows))
	for _, row := range rows {
		var consumers []*filament.DomainPositions
		if err := json.Unmarshal(row.ConsumerPositions, &consumers); err != nil {
			return nil, fmt.Errorf("decode webhook consumer positions: %w", err)
		}
		release := inbox.Release{
			Tenant: row.TenantID, Endpoint: row.SourceConnectionID,
			Orphaned: row.Deleted || len(consumers) == 0,
		}
		if !release.Orphaned {
			domain := inbox.Domain(row.SourceConnectionID, stream, incarnation)
			for n, positions := range consumers {
				certified, err := certifiedSequence(positions, domain)
				if err != nil {
					return nil, err
				}
				if n == 0 || certified < release.Through {
					release.Through = certified
				}
			}
		}
		releases = append(releases, release)
	}
	return releases, nil
}

// DeleteWebhookEndpointsOfDeletedSources removes the endpoint rows of sources
// deleted before the given time. Call it only after retention has purged every
// delivery accepted before then.
func (s *Store) DeleteWebhookEndpointsOfDeletedSources(ctx context.Context, before time.Time) (int64, error) {
	return s.q.DeleteWebhookEndpointsOfDeletedSources(ctx, pgtype.Timestamptz{Time: before, Valid: true})
}

// certifiedSequence is a consumer's certified inbox sequence, or zero when it
// has none for this inbox incarnation.
func certifiedSequence(positions *filament.DomainPositions, domain filament.DomainKey) (uint64, error) {
	if positions == nil {
		return 0, nil
	}
	position, ok := (*positions)[domain]
	if !ok || position.Codec != inbox.PositionCodec || position.Version != 0 {
		return 0, nil
	}
	return strconv.ParseUint(string(position.Value), 10, 64)
}

// LoadWebhookEndpointForTenant returns an endpoint only when it belongs to tenant.
func (s *Store) LoadWebhookEndpointForTenant(ctx context.Context, tenant filament.TenantID, sourceID string) (webhooks.Endpoint, error) {
	row, err := s.q.GetWebhookEndpointBySource(ctx, sqlcgen.GetWebhookEndpointBySourceParams{
		TenantID: string(tenant), SourceConnectionID: sourceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return webhooks.Endpoint{}, fmt.Errorf("webhook endpoint %q: %w", sourceID, filament.ErrNotFound)
	}
	if err != nil {
		return webhooks.Endpoint{}, fmt.Errorf("load webhook endpoint: %w", err)
	}
	return endpointFromRow(row), nil
}

// PutWebhookEndpoint persists receiver configuration under an optimistic
// revision. An enabled endpoint need not have a ready route. The receiver
// answers 503 while no pipeline reads the source and stores deliveries once
// one does, and routeReady reports whether the bound route reads them all.
func (s *Store) PutWebhookEndpoint(ctx context.Context, mutation webhooks.EndpointMutation) (webhooks.Endpoint, error) {
	if err := mutation.Validate(); err != nil {
		return webhooks.Endpoint{}, err
	}
	resources := mutation.Resources
	// Validate restricts success status to 200, 202, or 204, all within int16.
	// #nosec G115
	successStatus := int16(mutation.SuccessStatus)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return webhooks.Endpoint{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if mutation.ExpectedPipelineVersionID != "" {
		var currentVersion pgtype.Text
		err = tx.QueryRow(ctx, `SELECT current_version_id::text FROM pipelines
			WHERE tenant_id = $1 AND id = $2 AND NOT is_deleted FOR UPDATE`,
			string(mutation.Tenant), mutation.PipelineID).Scan(&currentVersion)
		if errors.Is(err, pgx.ErrNoRows) {
			return webhooks.Endpoint{}, webhooks.ErrRouteNotReady
		}
		if err != nil {
			return webhooks.Endpoint{}, fmt.Errorf("load current pipeline version: %w", err)
		}
		if !currentVersion.Valid || currentVersion.String != mutation.ExpectedPipelineVersionID {
			return webhooks.Endpoint{}, filament.ErrVersionConflict
		}
	}
	q := s.q.WithTx(tx)
	pipeline := pgtype.Text{String: mutation.PipelineID, Valid: mutation.PipelineID != ""}
	route := pgtype.Text{String: mutation.RouteKey, Valid: mutation.RouteKey != ""}
	expires := pgtype.Timestamptz{Time: mutation.PreviousKeyExpiresAt, Valid: !mutation.PreviousKeyExpiresAt.IsZero()}
	var n int64
	if mutation.ExpectedRevision == 0 {
		n, err = q.InsertWebhookEndpoint(ctx, sqlcgen.InsertWebhookEndpointParams{
			SourceConnectionID: mutation.SourceID, TenantID: string(mutation.Tenant),
			Verifier: mutation.Verifier, PipelineID: pipeline, RouteKey: route, Enabled: mutation.Enabled,
			SecretRefs: mutation.SecretRefs, PreviousKeyExpiresAt: expires, Resources: resources,
			SuccessStatus: successStatus, MaxBodyBytes: mutation.MaxBodyBytes,
		})
	} else {
		// The store owns revisions: each update advances by one and stamps its time.
		n, err = q.UpdateWebhookEndpoint(ctx, sqlcgen.UpdateWebhookEndpointParams{
			SourceConnectionID: mutation.SourceID, TenantID: string(mutation.Tenant),
			ExpectedRevision: mutation.ExpectedRevision, NextRevision: mutation.ExpectedRevision + 1,
			UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, Verifier: mutation.Verifier,
			PipelineID: pipeline, RouteKey: route, Enabled: mutation.Enabled,
			SecretRefs: mutation.SecretRefs, PreviousKeyExpiresAt: expires, Resources: resources,
			SuccessStatus: successStatus, MaxBodyBytes: mutation.MaxBodyBytes,
		})
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return webhooks.Endpoint{}, webhooks.ErrRouteNotReady
		}
		return webhooks.Endpoint{}, fmt.Errorf("persist webhook endpoint: %w", err)
	}
	if n != 1 {
		return webhooks.Endpoint{}, filament.ErrVersionConflict
	}
	row, err := q.GetWebhookEndpointBySource(ctx, sqlcgen.GetWebhookEndpointBySourceParams{
		TenantID: string(mutation.Tenant), SourceConnectionID: mutation.SourceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return webhooks.Endpoint{}, filament.ErrNotFound
	}
	if err != nil {
		return webhooks.Endpoint{}, err
	}
	endpoint := endpointFromRow(row)
	if err := tx.Commit(ctx); err != nil {
		return webhooks.Endpoint{}, err
	}
	return endpoint, nil
}

// LoadWebhookEndpoint resolves an opaque, globally unique source connection
// ID to its tenant-owned endpoint. The receiver never accepts tenant identity
// from a delivery.
func (s *Store) LoadWebhookEndpoint(ctx context.Context, sourceID string) (webhooks.Endpoint, error) {
	tenant, err := s.q.GetWebhookEndpointTenant(ctx, sourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return webhooks.Endpoint{}, fmt.Errorf("webhook endpoint %q: %w", sourceID, filament.ErrNotFound)
	}
	if err != nil {
		return webhooks.Endpoint{}, fmt.Errorf("load webhook endpoint: %w", err)
	}
	return s.LoadWebhookEndpointForTenant(ctx, filament.TenantID(tenant), sourceID)
}

// routeReads reports whether the bound route is live, continuous, and reads
// every configured resource.
func routeReads(row *sqlcgen.GetWebhookEndpointBySourceRow) bool {
	if len(row.Resources) == 0 || !row.PipelineID.Valid || !row.RouteKey.Valid || !row.RouteDesiredState.Valid ||
		row.RouteDesiredState.String == string(filament.StreamStopped) || len(row.RunSpec) == 0 {
		return false
	}
	var spec filament.RunSpec
	if err := json.Unmarshal(row.RunSpec, &spec); err != nil {
		return false
	}
	if spec.Tenant != filament.TenantID(row.TenantID) || spec.PipelineID != row.PipelineID.String ||
		spec.CheckpointRoute != row.RouteKey.String || spec.SourceConnectionID != row.SourceConnectionID ||
		spec.Options.Execution != filament.ExecutionContinuous {
		return false
	}
	for _, name := range row.Resources {
		if !slices.Contains(spec.Resources, name) {
			return false
		}
	}
	return true
}

func endpointFromRow(row *sqlcgen.GetWebhookEndpointBySourceRow) webhooks.Endpoint {
	endpoint := webhooks.Endpoint{
		Tenant: filament.TenantID(row.TenantID), SourceID: row.SourceConnectionID,
		Verifier: row.Verifier, Enabled: row.Enabled, Revision: row.ConfigRevision,
		PipelineID: row.PipelineID.String, RouteKey: row.RouteKey.String,
		RouteReady: routeReads(row), Accepting: row.Accepting, Resources: row.Resources,
		SecretRefs: row.SecretRefs, MaxBodyBytes: row.MaxBodyBytes,
		SuccessStatus: int(row.SuccessStatus),
	}
	if row.PreviousKeyExpiresAt.Valid {
		endpoint.PreviousKeyExpiresAt = row.PreviousKeyExpiresAt.Time
	}
	return endpoint
}
