package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/galaxy-io/filament/datastore/sqlite/sqlcgen"
)

func streamQueryFixture(t *testing.T) (*sql.DB, *sqlcgen.Queries) {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for _, stmt := range []string{
		`INSERT INTO tenants (id, created_at, updated_at) VALUES ('tenant', 0, 0)`,
		`INSERT INTO connections (id, tenant_id, kind, name, connector, created_at, updated_at) VALUES ('source', 'tenant', 'source', 'source', 'test', 0, 0), ('sink', 'tenant', 'sink', 'sink', 'test', 0, 0)`,
		`INSERT INTO pipelines (id, tenant_id, name, created_at, updated_at) VALUES ('pipeline', 'tenant', 'pipeline', 0, 0)`,
		`INSERT INTO pipeline_versions (id, tenant_id, pipeline_id, version, graph, created_at, updated_at) VALUES ('version', 'tenant', 'pipeline', 1, '{}', 0, 0), ('next', 'tenant', 'pipeline', 2, '{}', 0, 0)`,
		`INSERT INTO runs (id, tenant_id, pipeline_id, pipeline_version_id, status, request, created_at, updated_at) VALUES ('run', 'tenant', 'pipeline', 'version', 0, '{}', 0, 0)`,
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	_, err = s.q.CreateReplicationStream(t.Context(), sqlcgen.CreateReplicationStreamParams{
		ReplicationStreamID: "stream", TenantID: "tenant", PipelineID: "pipeline", RouteKey: "route", Generation: 1,
		SourceConnectionID: "source", SinkConnectionID: "sink", ConsumerName: "consumer", ConsumerConfig: `{"keep":true}`, ContinuityFingerprint: "fingerprint", CreatedFromPipelineVersionID: "version", CreatedAt: 10, UpdatedAt: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s.db, s.q
}

func streamQueryOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func streamQueryRows(t *testing.T, got int64, err error, want int64) {
	t.Helper()
	streamQueryOK(t, err)
	if got != want {
		t.Fatalf("affected rows = %d, want %d", got, want)
	}
}
func streamQueryString(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }

func TestStreamQueriesMembership(t *testing.T) {
	db, q := streamQueryFixture(t)
	ctx := t.Context()
	args := sqlcgen.UpsertReplicationStreamResourceParams{ResourceID: "resource", ReplicationStreamID: "stream", TenantID: "tenant", ResourceName: "table", BootstrapMode: "snapshot", BootstrapConfig: "{}", CreatedAt: 20, UpdatedAt: 20}
	streamQueryOK(t, q.UpsertReplicationStreamResource(ctx, args))
	_, err := db.Exec(`UPDATE replication_stream_resources SET status = 2, bootstrap_run_id = 'run', activated_at = 30, error = 'old error'`)
	streamQueryOK(t, err)
	// Keeping a selected resource must preserve its activation and identity.
	streamQueryOK(t, q.RetireReplicationStreamResources(ctx, sqlcgen.RetireReplicationStreamResourcesParams{ReplicationStreamID: "stream", ResourceNames: `["table"]`, UpdatedAt: 40, RetiredAt: sql.NullInt64{Int64: 40, Valid: true}}))
	args.ResourceID = "unused-new-id"
	args.UpdatedAt = 40
	streamQueryOK(t, q.UpsertReplicationStreamResource(ctx, args))
	rows, err := q.ListReplicationStreamResources(ctx, "stream")
	streamQueryOK(t, err)
	if len(rows) != 1 || rows[0].ID != "resource" || rows[0].Status != 2 || rows[0].ActivatedAt.Int64 != 30 {
		t.Fatalf("membership was not preserved: %+v", rows)
	}
	streamQueryOK(t, q.RetireReplicationStreamResources(ctx, sqlcgen.RetireReplicationStreamResourcesParams{ReplicationStreamID: "stream", ResourceNames: `[]`, UpdatedAt: 50, RetiredAt: sql.NullInt64{Int64: 50, Valid: true}}))
	streamQueryOK(t, q.UpsertReplicationStreamResource(ctx, args))
	rows, err = q.ListReplicationStreamResources(ctx, "stream")
	streamQueryOK(t, err)
	r := rows[0]
	if r.Status != 0 || r.RetiredAt.Valid || r.ActivatedAt.Valid || r.BootstrapRunID.Valid || r.Error.Valid {
		t.Fatalf("retired resource was not reset: %+v", r)
	}
	streamQueryOK(t, q.RetireReplicationStream(ctx, sqlcgen.RetireReplicationStreamParams{ReplicationStreamID: "stream", UpdatedAt: 60, RetiredAt: sql.NullInt64{Int64: 60, Valid: true}}))
	listArgs := sqlcgen.ListRetiredReplicationStreamsForRouteParams{PipelineID: "pipeline", RouteKey: "route"}
	streams, err := q.ListRetiredReplicationStreamsForRoute(ctx, listArgs)
	streamQueryOK(t, err)
	if len(streams) != 1 {
		t.Fatal("uncleaned stream missing")
	}
	streamQueryOK(t, q.MarkReplicationStreamCleaned(ctx, sqlcgen.MarkReplicationStreamCleanedParams{ReplicationStreamID: "stream", UpdatedAt: 70}))
	streams, err = q.ListRetiredReplicationStreamsForRoute(ctx, listArgs)
	streamQueryOK(t, err)
	if len(streams) != 0 {
		t.Fatal("cleaned stream still listed")
	}
	var kept, cleaned string
	streamQueryOK(t, db.QueryRow(`SELECT json_type(consumer_config, '$.keep'), json_type(consumer_config, '$._filament_cleanup_complete') FROM replication_streams`).Scan(&kept, &cleaned))
	if kept != "true" || cleaned != "true" {
		t.Fatal("cleanup must preserve config and set a JSON boolean")
	}
	generation, err := q.NextReplicationStreamGeneration(ctx, sqlcgen.NextReplicationStreamGenerationParams{PipelineID: "pipeline", RouteKey: "route"})
	streamQueryOK(t, err)
	if generation != 2 {
		t.Fatalf("next generation = %d", generation)
	}
}

func TestStreamQueriesRuntime(t *testing.T) {
	db, q := streamQueryFixture(t)
	ctx := t.Context()
	request, err := q.LoadRunRequest(ctx, sqlcgen.LoadRunRequestParams{RunID: "run", TenantID: "tenant", PipelineID: streamQueryString("pipeline"), PipelineVersionID: streamQueryString("version")})
	streamQueryOK(t, err)
	if request != "{}" {
		t.Fatalf("request = %s", request)
	}
	streamQueryOK(t, q.InitializeStreamExecution(ctx, sqlcgen.InitializeStreamExecutionParams{RunID: streamQueryString("run"), RunSpec: streamQueryString(`{"PipelineVersionID":"version"}`), StreamID: "stream", TenantID: "tenant"}))
	execution, err := q.GetStreamExecution(ctx, sqlcgen.GetStreamExecutionParams{StreamID: "stream", TenantID: "tenant"})
	streamQueryOK(t, err)
	if execution.RunID != "run" || execution.PipelineVersionID != "version" || execution.Revision != 1 {
		t.Fatalf("execution: %+v", execution)
	}
	n, err := q.ChangeStreamDesiredState(ctx, sqlcgen.ChangeStreamDesiredStateParams{StreamID: "stream", TenantID: "tenant", Revision: 1, DesiredState: "paused"})
	streamQueryRows(t, n, err, 1)
	n, err = q.ChangeStreamDesiredState(ctx, sqlcgen.ChangeStreamDesiredStateParams{StreamID: "stream", TenantID: "tenant", Revision: 1, DesiredState: "enabled"})
	streamQueryRows(t, n, err, 0)
	ids, err := q.ListReconcilableStreamIDs(ctx, sqlcgen.ListReconcilableStreamIDsParams{TenantID: "tenant", PageLimit: 10})
	streamQueryOK(t, err)
	if len(ids) != 1 || ids[0] != "stream" {
		t.Fatalf("reconcile IDs: %v", ids)
	}
	attempt, err := q.CreateStreamAttempt(ctx, sqlcgen.CreateStreamAttemptParams{StreamID: "stream", TenantID: "tenant", ExecutionID: "first", DesiredRevision: 2, TtlUs: 60000000})
	streamQueryOK(t, err)
	if attempt.RequestTtlUs != 60000000 || attempt.ExpiresAt-attempt.StartedAt != 60000 {
		t.Fatalf("TTL conversion: %+v", attempt)
	}
	n, err = q.RenewStreamAttempt(ctx, sqlcgen.RenewStreamAttemptParams{Token: attempt.Token, TenantID: "tenant", TtlUs: 1})
	streamQueryRows(t, n, err, 1)
	renewed, err := q.GetStreamAttemptByExecutionID(ctx, sqlcgen.GetStreamAttemptByExecutionIDParams{TenantID: "tenant", ExecutionID: "first"})
	streamQueryOK(t, err)
	if renewed.ExpiresAt < attempt.ExpiresAt {
		t.Fatal("renew shortened lease")
	}
	epoch, err := q.CreateStreamEpoch(ctx, sqlcgen.CreateStreamEpochParams{StreamID: "stream", TenantID: "tenant", Epoch: 1, AttemptToken: attempt.Token, Certificate: []byte("proof"), Positions: `{"table":1}`})
	streamQueryOK(t, err)
	if string(epoch.Certificate) != "proof" {
		t.Fatal("certificate did not round trip")
	}
	n, err = q.AdvanceStreamEpoch(ctx, sqlcgen.AdvanceStreamEpochParams{StreamID: "stream", TenantID: "tenant", Epoch: 1, PreviousEpoch: 0})
	streamQueryRows(t, n, err, 1)
	n, err = q.AdvanceStreamEpoch(ctx, sqlcgen.AdvanceStreamEpochParams{StreamID: "stream", TenantID: "tenant", Epoch: 2, PreviousEpoch: 0})
	streamQueryRows(t, n, err, 0)
	positions, err := q.LatestStreamPositions(ctx, sqlcgen.LatestStreamPositionsParams{StreamID: "stream", TenantID: "tenant"})
	streamQueryOK(t, err)
	if positions != epoch.Positions {
		t.Fatal("latest positions mismatch")
	}
	_, err = db.Exec(`UPDATE stream_attempts SET expires_at = 0`)
	streamQueryOK(t, err)
	n, err = q.RenewStreamAttempt(ctx, sqlcgen.RenewStreamAttemptParams{Token: attempt.Token, TenantID: "tenant", TtlUs: 60000000})
	streamQueryRows(t, n, err, 0)
	end := sqlcgen.EndStreamAttemptParams{Token: attempt.Token, TenantID: "tenant", Termination: streamQueryString("reaped"), RequireLive: true}
	n, err = q.EndStreamAttempt(ctx, end)
	streamQueryRows(t, n, err, 0)
	end.RequireLive = false
	n, err = q.EndStreamAttempt(ctx, end)
	streamQueryRows(t, n, err, 1)
	tiny, err := q.CreateStreamAttempt(ctx, sqlcgen.CreateStreamAttemptParams{StreamID: "stream", TenantID: "tenant", ExecutionID: "tiny", DesiredRevision: 2, TtlUs: 1})
	streamQueryOK(t, err)
	if tiny.Token <= attempt.Token || tiny.ExpiresAt-tiny.StartedAt != 1 || tiny.RequestTtlUs != 1 {
		t.Fatalf("tiny TTL/token: %+v", tiny)
	}
}

func TestStreamQueriesCheckpointTransaction(t *testing.T) {
	db, q := streamQueryFixture(t)
	ctx := t.Context()
	streamQueryOK(t, q.UpsertReplicationStreamResource(ctx, sqlcgen.UpsertReplicationStreamResourceParams{ResourceID: "resource", ReplicationStreamID: "stream", TenantID: "tenant", ResourceName: "table", BootstrapMode: "snapshot", BootstrapConfig: "{}"}))
	// A version-scoped cursor must be replaced when a stream takes ownership.
	_, err := q.SaveResourceCheckpoint(ctx, sqlcgen.SaveResourceCheckpointParams{TenantID: "tenant", PipelineID: "pipeline", PipelineVersionID: "version", RouteKey: "route", ResourceName: "table", Cursor: "old", LastRunID: "run"})
	streamQueryOK(t, err)
	save := func(ctx context.Context, version, cursor string, commit bool) {
		t.Helper()
		tx, err := db.BeginTx(ctx, nil)
		streamQueryOK(t, err)
		defer tx.Rollback()
		tq := q.WithTx(tx)
		_, err = tq.LockReplicationStreamGeneration(ctx, sqlcgen.LockReplicationStreamGenerationParams{StreamID: "stream", TenantID: "tenant", Generation: 1})
		streamQueryOK(t, err)
		resource, err := tq.GetStreamResourceForCheckpoint(ctx, sqlcgen.GetStreamResourceForCheckpointParams{ReplicationStreamID: "stream", TenantID: "tenant", ResourceName: "table"})
		streamQueryOK(t, err)
		if resource.Status == 3 {
			t.Fatal("cannot save retired resource")
		}
		streamQueryOK(t, tq.DeleteConflictingStreamResourceCheckpoint(ctx, sqlcgen.DeleteConflictingStreamResourceCheckpointParams{PipelineID: "pipeline", PipelineVersionID: version, RouteKey: "route", ResourceName: "table", ResourceID: streamQueryString(resource.ID)}))
		if resource.Status != 2 {
			streamQueryOK(t, tq.BumpReplicationStreamMembershipRevision(ctx, sqlcgen.BumpReplicationStreamMembershipRevisionParams{ReplicationStreamID: "stream", TenantID: "tenant"}))
		}
		n, err := tq.ActivateStreamResourceForCheckpoint(ctx, sqlcgen.ActivateStreamResourceForCheckpointParams{ResourceID: resource.ID, TenantID: "tenant", LastRunID: streamQueryString("run"), ActivatedAt: sql.NullInt64{Int64: 100, Valid: true}, UpdatedAt: 100})
		streamQueryRows(t, n, err, 1)
		n, err = tq.SaveStreamResourceCheckpoint(ctx, sqlcgen.SaveStreamResourceCheckpointParams{ResourceID: resource.ID, TenantID: "tenant", PipelineID: "pipeline", PipelineVersionID: version, RouteKey: "route", Cursor: cursor, LastRunID: "run", CreatedAt: 100, UpdatedAt: 100})
		streamQueryRows(t, n, err, 1)
		if commit {
			streamQueryOK(t, tx.Commit())
		}
	}
	save(ctx, "version", "rolled back", false)
	s, err := q.GetReplicationStream(ctx, "stream")
	streamQueryOK(t, err)
	if s.MembershipRevision != 1 {
		t.Fatal("rollback changed membership")
	}
	save(ctx, "version", "first", true)
	save(ctx, "next", "second", true)
	rows, err := q.ListStreamResourceCheckpoints(ctx, "stream")
	streamQueryOK(t, err)
	if len(rows) != 1 || rows[0].Cursor != "second" {
		t.Fatalf("cross-version checkpoints: %+v", rows)
	}
	s, err = q.GetReplicationStream(ctx, "stream")
	streamQueryOK(t, err)
	if s.MembershipRevision != 2 {
		t.Fatalf("membership revision = %d", s.MembershipRevision)
	}
	tx, err := db.BeginTx(ctx, nil)
	streamQueryOK(t, err)
	defer tx.Rollback()
	tq := q.WithTx(tx)
	streamQueryOK(t, tq.DeleteStreamResourceCheckpoint(ctx, sqlcgen.DeleteStreamResourceCheckpointParams{ReplicationStreamID: "stream", TenantID: "tenant", ResourceName: "table"}))
	streamQueryOK(t, tq.BumpReplicationStreamMembershipRevision(ctx, sqlcgen.BumpReplicationStreamMembershipRevisionParams{ReplicationStreamID: "stream", TenantID: "tenant"}))
	n, err := tq.ResetStreamResourceForCheckpoint(ctx, sqlcgen.ResetStreamResourceForCheckpointParams{ResourceID: "resource", TenantID: "tenant", UpdatedAt: 200})
	streamQueryRows(t, n, err, 1)
	streamQueryOK(t, tx.Commit())
	_, err = q.LoadStreamResourceCheckpoint(ctx, sqlcgen.LoadStreamResourceCheckpointParams{ReplicationStreamID: "stream", ResourceName: "table"})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted cursor: %v", err)
	}
}
