package sink

import (
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
)

func TestResolveStreamAndLimits(t *testing.T) {
	cfg, err := resolve(filament.NewConfig(map[string]any{"uri": "rabbitmq-stream://guest:guest@localhost:5552/%2f", "stream": "events.{resource}"}))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.stream.perResource() || cfg.stream.expand("orders") != "events.orders" {
		t.Fatalf("template: %q", cfg.stream)
	}
	if cfg.timeout.String() != "30s" || cfg.maxInFlight != 1024 || cfg.maxRecordBytes != 1<<20 {
		t.Fatalf("defaults: %+v", cfg)
	}
	if _, err := resolve(filament.NewConfig(map[string]any{"stream": ""})); err == nil {
		t.Fatal("missing stream accepted")
	}
	if _, err := resolve(filament.NewConfig(map[string]any{"stream": "bad name"})); err == nil {
		t.Fatal("blank stream name accepted")
	}
	if _, err := resolve(filament.NewConfig(map[string]any{"stream": "events", "publish_timeout": "5ms"})); err == nil || !strings.Contains(err.Error(), "publish_timeout") {
		t.Fatal(err)
	}
	if _, err := resolve(filament.NewConfig(map[string]any{"stream": "events", "max_record_bytes": 10})); err == nil {
		t.Fatal("tiny record limit accepted")
	}
}

func TestProducerNameIgnoresRunIdentity(t *testing.T) {
	a := producerName("tenant", "pipeline", "connection", "orders")
	b := producerName("tenant", "pipeline", "connection", "orders")
	if a != b || !strings.HasPrefix(a, "filament-") || len(a) != len("filament-")+16 {
		t.Fatalf("name %q", a)
	}
	if producerName("tenant", "pipeline", "connection", "other") == a {
		t.Fatal("stream did not change the producer name")
	}
}
