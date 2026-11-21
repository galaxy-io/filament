package alchemy

import (
	"testing"

	"github.com/galaxy-io/filament/datastore/sqlite"
)

func TestFromEnvDisabledWithoutURL(t *testing.T) {
	t.Setenv("GALAXY_ALCHEMY_SERVER_URL", "")
	t.Setenv("GALAXY_ALCHEMY_URL", "")
	cfg, err := FromEnv(sqlite.NewMemory())
	if err != nil || cfg.Client != nil || cfg.Functions != nil {
		t.Fatalf("FromEnv() = %+v, %v; want a disabled config with nil Functions", cfg, err)
	}
}
