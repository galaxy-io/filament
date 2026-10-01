package connectors

import (
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

func TestRedshiftSinkIsRegistered(t *testing.T) {
	spec, err := registry.DefaultSinks.Spec("redshift")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "redshift" || spec.DisplayName != "Amazon Redshift" || spec.Maturity != filament.MaturityAlpha {
		t.Fatalf("Redshift sink spec = %#v", spec)
	}
}

func TestKafkaSourceIsRegistered(t *testing.T) {
	spec, err := registry.DefaultSources.Spec("kafka")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "kafka" || spec.DisplayName != "Kafka" || spec.Stream == nil || spec.Stream.Input != filament.InputMessages || len(spec.SourcePolicies) == 0 || len(spec.Modes) != 1 || spec.Modes[0] != filament.ModeFull {
		t.Fatalf("Kafka source spec = %#v", spec)
	}
}
