package main

import (
	"context"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament"
	webhooksource "github.com/galaxy-io/filament/connectors/webhook"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/webhooks/inbox"
)

func TestStandaloneWebhooksUseEmbeddedInbox(t *testing.T) {
	ctx := context.Background()
	server, err := natsserver.NewServer(&natsserver.Options{
		DontListen: true, JetStream: true, StoreDir: t.TempDir(), MaxPayload: standaloneNATSMaxPayload,
	})
	if err != nil {
		t.Fatal(err)
	}
	go server.Start()
	defer server.Shutdown()
	if !server.ReadyForConnections(5 * time.Second) {
		t.Fatal("embedded NATS did not start")
	}
	conn, err := nats.Connect("", nats.InProcessServer(server))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	config := standaloneWebhookConfig{
		enabled:     true,
		inbox:       inbox.Config{MaxBytes: 8 << 20, MaxMsgSize: 2 << 20, Replicas: 1},
		maxInFlight: 2,
	}
	i, retention, err := openStandaloneWebhooks(ctx, conn, config)
	if err != nil || i == nil || retention == nil {
		t.Fatalf("open standalone webhooks: %v", err)
	}
	base := registry.NewSources()
	base.Register("webhook", func() filament.Source { return webhooksource.New(nil) })
	source, err := (standaloneSources{SourceRegistry: base, inbox: i}).Resolve("webhook")
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Configure(ctx, filament.NewConfig(nil)); err != nil {
		t.Fatalf("source could not use embedded inbox: %v", err)
	}
}
