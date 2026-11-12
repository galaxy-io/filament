package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite/sqlcgen"
	"github.com/galaxy-io/filament/webhooks"
	"github.com/galaxy-io/filament/webhooks/inbox"
)

var (
	_ webhooks.EndpointLoader = (*Store)(nil)
	_ webhooks.EndpointStore  = (*Store)(nil)
)

// LoadWebhookEndpoint resolves a source ID for the receiver. Tenant identity
// comes from the stored endpoint, never from the delivery request.
func (s *Store) LoadWebhookEndpoint(ctx context.Context, sourceID string) (webhooks.Endpoint, error) {
	row, err := s.q.GetWebhookEndpointBySource(ctx, sourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return webhooks.Endpoint{}, fmt.Errorf("webhook endpoint %q: %w", sourceID, filament.ErrNotFound)
	}
	if err != nil {
		return webhooks.Endpoint{}, fmt.Errorf("load webhook endpoint: %w", err)
	}
	return sqliteWebhookEndpoint(row)
}

// LoadWebhookEndpointForTenant enforces the API caller's tenant.
func (s *Store) LoadWebhookEndpointForTenant(ctx context.Context, tenant filament.TenantID, sourceID string) (webhooks.Endpoint, error) {
	endpoint, err := s.LoadWebhookEndpoint(ctx, sourceID)
	if err != nil {
		return webhooks.Endpoint{}, err
	}
	if endpoint.Tenant != tenant {
		return webhooks.Endpoint{}, filament.ErrNotFound
	}
	return endpoint, nil
}

// PutWebhookEndpoint validates and writes one revision in a SQLite transaction.
func (s *Store) PutWebhookEndpoint(ctx context.Context, mutation webhooks.EndpointMutation) (webhooks.Endpoint, error) {
	if err := mutation.Validate(); err != nil {
		return webhooks.Endpoint{}, err
	}
	secretRefs, err := json.Marshal(mutation.SecretRefs)
	if err != nil {
		return webhooks.Endpoint{}, err
	}
	resources, err := json.Marshal(mutation.Resources)
	if err != nil {
		return webhooks.Endpoint{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return webhooks.Endpoint{}, fmt.Errorf("begin webhook endpoint update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var sourceExists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM connections
		WHERE id = ? AND tenant_id = ? AND kind = 'source' AND is_deleted = 0`,
		mutation.SourceID, string(mutation.Tenant)).Scan(&sourceExists)
	if errors.Is(err, sql.ErrNoRows) {
		return webhooks.Endpoint{}, filament.ErrNotFound
	}
	if err != nil {
		return webhooks.Endpoint{}, fmt.Errorf("load webhook source: %w", err)
	}
	if mutation.PipelineID != "" {
		var currentVersion sql.NullString
		err = tx.QueryRowContext(ctx, `SELECT current_version_id FROM pipelines
			WHERE id = ? AND tenant_id = ? AND is_deleted = 0`,
			mutation.PipelineID, string(mutation.Tenant)).Scan(&currentVersion)
		if errors.Is(err, sql.ErrNoRows) {
			return webhooks.Endpoint{}, webhooks.ErrRouteNotReady
		}
		if err != nil {
			return webhooks.Endpoint{}, fmt.Errorf("load current pipeline version: %w", err)
		}
		if mutation.ExpectedPipelineVersionID != "" && (!currentVersion.Valid || currentVersion.String != mutation.ExpectedPipelineVersionID) {
			return webhooks.Endpoint{}, filament.ErrVersionConflict
		}
	}
	q := s.q.WithTx(tx)
	pipeline := sql.NullString{String: mutation.PipelineID, Valid: mutation.PipelineID != ""}
	route := sql.NullString{String: mutation.RouteKey, Valid: mutation.RouteKey != ""}
	expires := nullMillis(mutation.SecondaryKeyExpiresAt)
	enabled := int64(0)
	if mutation.Enabled {
		enabled = 1
	}
	var updated int64
	if mutation.ExpectedRevision == 0 {
		updated, err = q.InsertWebhookEndpoint(ctx, sqlcgen.InsertWebhookEndpointParams{
			SourceConnectionID: mutation.SourceID, TenantID: string(mutation.Tenant),
			Verifier: mutation.Verifier, PipelineID: pipeline, RouteKey: route, Enabled: enabled,
			SecretRefs: string(secretRefs), SecondaryKeyExpiresAt: expires, Resources: string(resources),
			SuccessStatus: int64(mutation.SuccessStatus), MaxBodyBytes: mutation.MaxBodyBytes,
		})
	} else {
		updated, err = q.UpdateWebhookEndpoint(ctx, sqlcgen.UpdateWebhookEndpointParams{
			SourceConnectionID: mutation.SourceID, TenantID: string(mutation.Tenant),
			ExpectedRevision: mutation.ExpectedRevision, NextRevision: mutation.ExpectedRevision + 1,
			UpdatedAt: nowMillis(), Verifier: mutation.Verifier,
			PipelineID: pipeline, RouteKey: route, Enabled: enabled,
			SecretRefs: string(secretRefs), SecondaryKeyExpiresAt: expires, Resources: string(resources),
			SuccessStatus: int64(mutation.SuccessStatus), MaxBodyBytes: mutation.MaxBodyBytes,
		})
	}
	if err != nil {
		return webhooks.Endpoint{}, fmt.Errorf("persist webhook endpoint: %w", err)
	}
	if updated != 1 {
		return webhooks.Endpoint{}, filament.ErrVersionConflict
	}
	row, err := q.GetWebhookEndpointBySource(ctx, mutation.SourceID)
	if err != nil {
		return webhooks.Endpoint{}, fmt.Errorf("load written webhook endpoint: %w", err)
	}
	endpoint, err := sqliteWebhookEndpoint(row)
	if err != nil {
		return webhooks.Endpoint{}, err
	}
	if err := tx.Commit(); err != nil {
		return webhooks.Endpoint{}, fmt.Errorf("commit webhook endpoint: %w", err)
	}
	return endpoint, nil
}

func sqliteWebhookEndpoint(row *sqlcgen.GetWebhookEndpointBySourceRow) (webhooks.Endpoint, error) {
	var refs, resources []string
	if err := json.Unmarshal([]byte(row.SecretRefs), &refs); err != nil {
		return webhooks.Endpoint{}, fmt.Errorf("decode webhook signing keys: %w", err)
	}
	if err := json.Unmarshal([]byte(row.Resources), &resources); err != nil {
		return webhooks.Endpoint{}, fmt.Errorf("decode webhook resources: %w", err)
	}
	endpoint := webhooks.Endpoint{
		Tenant: filament.TenantID(row.TenantID), SourceID: row.SourceConnectionID,
		Verifier: row.Verifier, Enabled: row.Enabled != 0, Revision: row.ConfigRevision,
		PipelineID: row.PipelineID.String, RouteKey: row.RouteKey.String,
		Accepting: row.Accepting, Resources: resources, SecretRefs: refs,
		MaxBodyBytes: row.MaxBodyBytes, SuccessStatus: int(row.SuccessStatus),
	}
	if row.SecondaryKeyExpiresAt.Valid {
		endpoint.SecondaryKeyExpiresAt = millisTime(row.SecondaryKeyExpiresAt.Int64)
	}
	if len(resources) > 0 && row.PipelineID.Valid && row.RouteKey.Valid && row.RouteDesiredState.Valid &&
		row.RouteDesiredState.String != string(filament.StreamStopped) && row.RunSpec.Valid {
		var spec filament.RunSpec
		if json.Unmarshal([]byte(row.RunSpec.String), &spec) == nil &&
			spec.Tenant == endpoint.Tenant && spec.PipelineID == endpoint.PipelineID &&
			spec.CheckpointRoute == endpoint.RouteKey && spec.SourceConnectionID == endpoint.SourceID &&
			spec.Options.Execution == filament.ExecutionContinuous {
			endpoint.RouteReady = true
			for _, name := range resources {
				if !slices.Contains(spec.Resources, name) {
					endpoint.RouteReady = false
					break
				}
			}
		}
	}
	return endpoint, nil
}

// WebhookReleases reports the least certified position of each live consumer.
func (s *Store) WebhookReleases(ctx context.Context, stream string, incarnation time.Time) ([]inbox.Release, error) {
	rows, err := s.q.ListWebhookReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("list webhook releases: %w", err)
	}
	releases := make([]inbox.Release, 0)
	index := make(map[string]int)
	for _, row := range rows {
		n, ok := index[row.SourceConnectionID]
		if !ok {
			n = len(releases)
			index[row.SourceConnectionID] = n
			releases = append(releases, inbox.Release{
				Tenant: row.TenantID, Endpoint: row.SourceConnectionID,
				Orphaned: row.Deleted != 0 || !row.StreamID.Valid,
			})
		}
		release := &releases[n]
		if row.Deleted != 0 || !row.StreamID.Valid {
			continue
		}
		release.Orphaned = false
		var positions filament.DomainPositions
		if row.Positions.Valid {
			if err := json.Unmarshal([]byte(row.Positions.String), &positions); err != nil {
				return nil, fmt.Errorf("decode webhook consumer positions: %w", err)
			}
		}
		sequence, err := sqliteWebhookCertifiedSequence(positions, inbox.Domain(row.SourceConnectionID, stream, incarnation))
		if err != nil {
			return nil, err
		}
		if !ok || sequence < release.Through {
			release.Through = sequence
		}
	}
	return releases, nil
}

func sqliteWebhookCertifiedSequence(positions filament.DomainPositions, domain filament.DomainKey) (uint64, error) {
	position, ok := positions[domain]
	if !ok || position.Codec != inbox.PositionCodec || position.Version != 0 {
		return 0, nil
	}
	return strconv.ParseUint(string(position.Value), 10, 64)
}

// DeleteWebhookEndpointsOfDeletedSources forgets rows after their inbox age expires.
func (s *Store) DeleteWebhookEndpointsOfDeletedSources(ctx context.Context, before time.Time) (int64, error) {
	return s.q.DeleteWebhookEndpointsOfDeletedSources(ctx, sql.NullInt64{Int64: before.UnixMilli(), Valid: true})
}
