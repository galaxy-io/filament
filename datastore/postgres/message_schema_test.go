package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/galaxy-io/filament"
	natssource "github.com/galaxy-io/filament/connectors/nats/source"
	pgsink "github.com/galaxy-io/filament/connectors/postgres/sink"
	"github.com/galaxy-io/filament/runner"
	"github.com/galaxy-io/filament/schema"
	"github.com/galaxy-io/filament/streamkit"
)

func TestLiveNATSSchemaMigrationAndRestart(t *testing.T) {
	url := os.Getenv("FILAMENT_TEST_NATS_URL")
	if url == "" {
		t.Skip("set FILAMENT_TEST_NATS_URL and FILAMENT_TEST_POSTGRES_DSN")
	}
	for _, policy := range []string{"nullable_add", "strict"} {
		t.Run(policy, func(t *testing.T) {
			f := newRuntimeFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			nc, err := nats.Connect(url)
			if err != nil {
				t.Fatal(err)
			}
			defer nc.Close()
			js, err := jetstream.New(nc)
			if err != nil {
				t.Fatal(err)
			}
			name := "schema_" + uuid.NewString()[:8]
			_, err = js.CreateStream(ctx, jetstream.StreamConfig{Name: name, Subjects: []string{"events"}})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = js.DeleteStream(context.Background(), name) }()
			namespace := "live_" + uuid.NewString()[:8]
			defer func() {
				_, _ = f.store.Pool().Exec(context.Background(), "DROP SCHEMA IF EXISTS "+pgx.Identifier{namespace}.Sanitize()+" CASCADE")
			}()
			codecs := &streamkit.Registry{}
			if err := natssource.RegisterCodec(codecs); err != nil {
				t.Fatal(err)
			}
			f.store.ConfigureStreamCodecs(codecs)
			publish := func(payloads ...string) {
				t.Helper()
				for _, payload := range payloads {
					if _, err := js.Publish(ctx, "events", []byte(payload)); err != nil {
						t.Fatal(err)
					}
				}
			}
			run := func(records, epochs int) error {
				t.Helper()
				f.request.ExecutionID = uuid.NewString()
				a := f.start(t)
				if err := f.store.ClaimStreamAttempt(ctx, a.Lease); err != nil {
					t.Fatal(err)
				}
				spec := a.Spec
				spec.Source = filament.Ref{Connector: "nats", Config: map[string]any{"url": url}}
				spec.Sink = filament.Ref{Connector: "postgres", Config: map[string]any{"dsn": testDSN(t), "schema": namespace, "schema_evolution": policy}}
				spec.IngestionTypes = map[string]filament.IngestionType{"events": filament.IngestionFullAppend}
				plan, err := filament.PlanContinuousWrite(natssource.New().Spec(), pgsink.New().Spec(), filament.WriteAppend)
				if err != nil {
					t.Fatal(err)
				}
				spec.WritePolicies = map[string]filament.WritePolicy{"events": plan.Policy}
				return runner.RunContinuous(ctx, runner.ContinuousConfig{Enabled: true, Spec: spec, Store: f.store, Source: natssource.New(), Sink: pgsink.New(), Codecs: codecs, Boundary: filament.Boundary{MaxRecords: records, MaxWait: 50 * time.Millisecond}, LeaseTTL: time.Minute, DrainTimeout: time.Second, MaxEpochs: epochs})
			}
			state := func() (filament.StreamState, filament.EpochCertificate) {
				t.Helper()
				s, err := f.store.LoadStreamState(ctx, f.activation.StreamStateRequest)
				if err != nil {
					t.Fatal(err)
				}
				if s.LastEpoch == nil {
					t.Fatal("no committed epoch")
				}
				epoch, err := f.store.GetEpoch(ctx, filament.EpochLookup{Tenant: tenantA, Key: *s.LastEpoch})
				if err != nil {
					t.Fatal(err)
				}
				return s, epoch.Certificate
			}
			count := func(want int) {
				t.Helper()
				var n int
				if err := f.store.Pool().QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{namespace, "events"}.Sanitize()).Scan(&n); err != nil || n != want {
					t.Fatalf("rows=%d want=%d err=%v", n, want, err)
				}
			}
			ackFloor := func(want uint64) {
				t.Helper()
				stream, err := js.Stream(ctx, name)
				if err != nil {
					t.Fatal(err)
				}
				lister := stream.ListConsumers(ctx)
				found := false
				for info := range lister.Info() {
					found = true
					if info.AckFloor.Stream != want {
						t.Fatalf("ack floor=%d want=%d", info.AckFloor.Stream, want)
					}
				}
				if err := lister.Err(); err != nil {
					t.Fatal(err)
				}
				if !found {
					t.Fatal("consumer missing")
				}
			}
			if policy == "strict" {
				// The whole initial batch establishes a schema, even when its first
				// message omits a field. Later additions still block before writing.
				publish(`{"id":1}`, `{"id":2,"email":"a@example.com"}`)
				if err := run(2, 1); err != nil {
					t.Fatal(err)
				}
				publish(`{"id":3,"extra":true}`)
				if err := run(1, 1); !errors.Is(err, schema.ErrBlocked) {
					t.Fatalf("strict did not block: %v", err)
				}
				count(2)
				ackFloor(2)
				s, cert := state()
				if s.LastEpoch.Epoch != 1 || len(cert.MessageSchemas["events"].Fields) != 2 {
					t.Fatal("blocked batch advanced layout/progress")
				}
				return
			}
			publish(`{"id":1}`, `{"id":2,"email":"a@example.com"}`, `{"id":3}`)
			if err := run(1, 3); err != nil {
				t.Fatal(err)
			}
			count(3)
			ackFloor(3)
			_, cert := state()
			if cert.FormatVersion != filament.EpochMessageSchemaFormatVersion || len(cert.MessageSchemas["events"].Fields) != 2 {
				t.Fatalf("layout missing from certificate: %+v", cert)
			}
			var nulls int
			if err := f.store.Pool().QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{namespace, "events"}.Sanitize()+" WHERE email IS NULL").Scan(&nulls); err != nil || nulls != 2 {
				t.Fatalf("missing fields not null: %d %v", nulls, err)
			}
			// A fresh source, sink, and runner restore the layout from the datastore.
			publish(`{"id":4}`)
			if err := run(1, 1); err != nil {
				t.Fatalf("restart with omitted field: %v", err)
			}
			count(4)
			ackFloor(4)
			publish(`{"id":5,"email":"valid"}`, `{"id":6,"email":99}`)
			if err := run(2, 1); !errors.Is(err, schema.ErrBlocked) {
				t.Fatalf("incompatible batch did not block: %v", err)
			}
			count(4)
			ackFloor(4)
			s, last := state()
			if s.LastEpoch.Epoch != 4 || len(last.MessageSchemas["events"].Fields) != 2 {
				t.Fatal("failed batch advanced layout/progress")
			}
		})
	}
}
