//go:build integration

package cdc_test

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

func TestPostgresCDCManagedPublicationFollowsSlot(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.SharedPostgresCDC(t)
	if _, err := pg.Pool().Exec(ctx, `CREATE TABLE cdc_owned (id bigint PRIMARY KEY, name text NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": pg.DSN(), "slot_name": "filament_test_owned"})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(ctx) }()
	if err := src.ExtractChanges(ctx, &testutil.CollectSink{}, filament.ChangeExtractOpts{Resources: []string{"cdc_owned"}}); err != nil {
		t.Fatal(err)
	}
	count := func() (slots, publications int) {
		t.Helper()
		if err := pg.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM pg_replication_slots WHERE slot_name = $1), (SELECT count(*) FROM pg_publication WHERE pubname = $1)`, "filament_test_owned").Scan(&slots, &publications); err != nil {
			t.Fatal(err)
		}
		return slots, publications
	}
	if slots, publications := count(); slots != 1 || publications != 1 {
		t.Fatalf("stream must own a slot and a publication of its name: slot=%d publication=%d", slots, publications)
	}
	if err := src.CleanupReplicationStream(ctx, filament.ReplicationStream{ConsumerName: "filament_test_owned"}); err != nil {
		t.Fatal(err)
	}
	if slots, publications := count(); slots != 0 || publications != 1 {
		t.Fatalf("a stream that did not record ownership must keep its publication: slot=%d publication=%d", slots, publications)
	}
	owned := filament.ReplicationStream{ConsumerName: "filament_test_owned", ConsumerConfig: map[string]any{"manage_publication": true}}
	if err := src.CleanupReplicationStream(ctx, owned); err != nil {
		t.Fatal(err)
	}
	if slots, publications := count(); slots != 0 || publications != 0 {
		t.Fatalf("retired stream left slot=%d publication=%d", slots, publications)
	}
}

func TestPostgresCDCNamedManagedPublicationIsRejected(t *testing.T) {
	err := pgsource.New().Validate(filament.NewConfig(map[string]any{"dsn": "postgres://u:p@localhost/db", "publication": "shared"}))
	if err == nil {
		t.Fatal("a managed publication must not take a configured name")
	}
	if err := pgsource.New().Validate(filament.NewConfig(map[string]any{"dsn": "postgres://u:p@localhost/db", "manage_publication": false})); err == nil {
		t.Fatal("an unmanaged publication must be named")
	}
}
