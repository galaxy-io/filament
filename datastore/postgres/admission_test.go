package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func admissionFixture(t *testing.T) (*postgres.Store, filament.ReplicationStream, []string) {
	t.Helper()
	ctx := context.Background()
	s := newTestStore(t)
	for _, c := range []filament.Connection{
		{ID: connectionOne, Tenant: tenantA, Kind: filament.ConnectorKindSource, Name: "source", Connector: "postgres"},
		{ID: connectionTwo, Tenant: tenantA, Kind: filament.ConnectorKindSink, Name: "sink", Connector: "postgres"},
	} {
		if _, err := s.CreateConnection(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: pipelineOne, TenantId: tenantA, Name: "admission"}); err != nil {
		t.Fatal(err)
	}
	versions := []string{}
	for range 2 {
		v, err := s.CreatePipelineVersion(ctx, tenantA, pipelineOne, &ingestionv1.PipelineVersion{Graph: &ingestionv1.PipelineGraph{}})
		if err != nil {
			t.Fatal(err)
		}
		versions = append(versions, v.GetId())
	}
	return s, filament.ReplicationStream{ID: replicationOne, Tenant: tenantA, PipelineID: pipelineOne, Route: "route", SourceConnectionID: connectionOne, SinkConnectionID: connectionTwo, ConsumerName: "admission", ContinuityFingerprint: "same", CreatedFromPipelineVersionID: versions[0]}, versions
}
func admissionRun(version, route string, status filament.RunStatus) filament.RunState {
	return filament.RunState{Run: filament.RunID(uuid.NewString()), Tenant: tenantA, Status: status, Request: filament.RunRequest{Tenant: tenantA, PipelineID: pipelineOne, PipelineVersionID: version, CheckpointRoute: route}}
}
func TestStore_CrossVersionAdmissionRace(t *testing.T) {
	s, desired, versions := admissionFixture(t)
	ctx := context.Background()
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := range 2 {
		go func(i int) {
			<-start
			candidate := desired
			candidate.ID = uuid.NewString()
			candidate.ConsumerName = fmt.Sprintf("candidate%d", i)
			candidate.CreatedFromPipelineVersionID = versions[i]
			results <- s.CreateRunWithReplicationStream(ctx, admissionRun(versions[i], desired.Route, filament.RunRequested), candidate)
		}(i)
	}
	close(start)
	admitted, conflicts := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			admitted++
		} else if errors.Is(err, filament.ErrRunOverlap) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if admitted != 1 || conflicts != 1 {
		t.Fatalf("admitted=%d conflicts=%d", admitted, conflicts)
	}
}
func TestStore_CrossVersionSuccessorRollback(t *testing.T) {
	s, desired, versions := admissionFixture(t)
	ctx := context.Background()
	if err := s.CreateRunWithReplicationStream(ctx, admissionRun(versions[0], desired.Route, filament.RunRunning), desired); err != nil {
		t.Fatal(err)
	}
	next := desired
	next.ID = replicationTwo
	next.ConsumerName = "successor"
	next.ContinuityFingerprint = "changed"
	next.CreatedFromPipelineVersionID = versions[1]
	if err := s.CreateRunWithReplicationStream(ctx, admissionRun(versions[1], desired.Route, filament.RunRequested), next); !errors.Is(err, filament.ErrRunOverlap) {
		t.Fatalf("overlap: %v", err)
	}
	current, err := s.LoadReplicationStream(ctx, desired.ID)
	if err != nil || current.Status != filament.ReplicationStreamActive {
		t.Fatalf("predecessor: %+v %v", current, err)
	}
	if _, err := s.LoadReplicationStream(ctx, next.ID); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("successor persisted: %v", err)
	}
}
func TestStore_AdmissionStatusesAndBoundedCompatibility(t *testing.T) {
	for _, status := range []filament.RunStatus{filament.RunRequested, filament.RunRunning, filament.RunPaused, filament.RunScheduled, filament.RunCompleted, filament.RunFailed, filament.RunCanceled, filament.RunPartial} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			s, desired, versions := admissionFixture(t)
			ctx := context.Background()
			first := admissionRun(versions[0], desired.Route, status)
			if err := s.CreateRunWithReplicationStream(ctx, first, desired); err != nil {
				t.Fatal(err)
			}
			err := s.CreateRunWithReplicationStream(ctx, admissionRun(versions[1], desired.Route, filament.RunRequested), desired)
			active := status == filament.RunRequested || status == filament.RunRunning || status == filament.RunPaused
			if active && !errors.Is(err, filament.ErrRunOverlap) || !active && err != nil {
				t.Fatalf("status %v: %v", status, err)
			}
			if status == filament.RunScheduled {
				first.Status = filament.RunRequested
				if err := s.CreateRunWithReplicationStream(ctx, first, desired); !errors.Is(err, filament.ErrRunOverlap) {
					t.Fatalf("scheduled promotion: %v", err)
				}
			}
			for _, v := range versions {
				if err := s.CreateRun(ctx, admissionRun(v, "bounded", filament.RunRequested)); err != nil {
					t.Fatalf("bounded cross-version: %v", err)
				}
			}
			if err := s.CreateRun(ctx, admissionRun(versions[0], "bounded", filament.RunRequested)); !errors.Is(err, filament.ErrRunOverlap) {
				t.Fatalf("bounded same-version: %v", err)
			}
			other := desired
			other.ID = uuid.NewString()
			other.Route = "other"
			other.ConsumerName = "other"
			if err := s.CreateRunWithReplicationStream(ctx, admissionRun(versions[0], "other", filament.RunRequested), other); err != nil {
				t.Fatalf("unrelated route: %v", err)
			}
		})
	}
}

func TestAdmissionMigrationHistoricalRequests(t *testing.T) {
	data, err := os.ReadFile("migrations/00007_repair_stream_admission.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := strings.Split(string(data), "-- +goose Down")[0]
	for _, overlap := range []bool{false, true} {
		t.Run(fmt.Sprintf("overlap=%v", overlap), func(t *testing.T) {
			ctx := context.Background()
			conn, err := pgx.Connect(ctx, testDSN(t))
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close(ctx)
			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			// A transaction-local pre-00007 runs table isolates migration fixtures from
			// the real datastore and tests actual PostgreSQL constraint/index SQL.
			_, err = tx.Exec(ctx, `CREATE TEMP TABLE runs(id uuid PRIMARY KEY,pipeline_id uuid,status smallint,request jsonb); CREATE UNIQUE INDEX runs_active_replication_route_idx ON runs(pipeline_id,(request->>'CheckpointRoute')) WHERE status IN(0,1,5) AND nullif(request->>'ReplicationStreamID','') IS NOT NULL`)
			if err != nil {
				t.Fatal(err)
			}
			nested, _ := json.Marshal(filament.RunRequest{CheckpointRoute: "route", ReplicationStream: &filament.StreamRef{ID: replicationOne, Generation: 1}})
			secondRoute := "other"
			if overlap {
				secondRoute = "route"
			}
			_, err = tx.Exec(ctx, `INSERT INTO runs VALUES($1,$2,0,$3),($4,$2,1,jsonb_build_object('CheckpointRoute',$5::text,'ReplicationStreamID',$6::text))`, runOne, pipelineOne, nested, runHighWater, secondRoute, replicationOne)
			if err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, up)
			if overlap {
				if err == nil || !strings.Contains(err.Error(), runOne) || !strings.Contains(err.Error(), runHighWater) {
					t.Fatalf("missing actionable conflict: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var count int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM runs WHERE coalesce(nullif(request->'ReplicationStream'->>'ID',''),nullif(request->>'ReplicationStreamID',''))=$1`, replicationOne).Scan(&count); err != nil || count != 2 {
				t.Fatalf("historical requests %d: %v", count, err)
			}
			// Old JSON writers remain fenced, and terminal records do not block.
			if _, err := tx.Exec(ctx, `UPDATE runs SET status=2 WHERE id=$1`, runOne); err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO runs VALUES($1,$2,0,$3)`, uuid.NewString(), pipelineOne, nested); err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, `INSERT INTO runs VALUES($1,$2,0,$3)`, uuid.NewString(), pipelineOne, nested)
			var pgerr interface{ SQLState() string }
			if !errors.As(err, &pgerr) || pgerr.SQLState() != "23505" {
				t.Fatalf("old writer escaped fence: %v", err)
			}
		})
	}
}

func TestStore_AdmissionRejectsMismatchedIdentity(t *testing.T) {
	for _, field := range []string{"tenant", "pipeline", "route"} {
		t.Run(field, func(t *testing.T) {
			s, desired, versions := admissionFixture(t)
			run := admissionRun(versions[0], desired.Route, filament.RunRequested)
			switch field {
			case "tenant":
				desired.Tenant = filament.TenantID(uuid.NewString())
			case "pipeline":
				desired.PipelineID = uuid.NewString()
			case "route":
				desired.Route = "different"
			}
			err := s.CreateRunWithReplicationStream(context.Background(), run, desired)
			if err == nil || !strings.Contains(err.Error(), "does not match run route identity") {
				t.Fatalf("identity mismatch: %v", err)
			}
			var count int
			if err := s.Pool().QueryRow(context.Background(), "SELECT count(*) FROM replication_streams").Scan(&count); err != nil || count != 0 {
				t.Fatalf("mismatch created stream: count=%d err=%v", count, err)
			}
		})
	}
}
