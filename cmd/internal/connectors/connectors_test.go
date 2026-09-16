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
