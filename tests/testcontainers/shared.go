//go:build integration

package testcontainers

import (
	"context"
	"os"
	"sync"
	"testing"
)

// Shared is a suite-wide container reset to pristine between tests. Acquire
// through the Shared* helpers, which boot on first use, wipe on every
// acquisition, and leave termination to the reaper at process exit. Tests
// sharing an instance must not run in parallel.
type Shared interface {
	Wipe(t testing.TB)
}

var (
	_ Shared = (*PG)(nil)
	_ Shared = (*K3s)(nil)
)

var (
	pgShared    sharedPG
	pgSharedCDC sharedPG
	k3sShared   struct {
		once    sync.Once
		cluster *K3s
	}
)

type sharedPG struct {
	once sync.Once
	pg   *PG
}

// SharedPostgres returns the suite-wide Postgres. Tests needing an exclusive
// or specially configured container keep Postgres(t).
func SharedPostgres(t testing.TB) *PG {
	t.Helper()
	return pgShared.acquire(t)
}

// SharedPostgresCDC is the suite-wide wal_level=logical Postgres for CDC
// tests; its wipe also clears the replication slots they leave behind.
func SharedPostgresCDC(t testing.TB) *PG {
	t.Helper()
	return pgSharedCDC.acquire(t, WithImage("postgres:16-alpine"), WithLogicalReplication())
}

func (s *sharedPG) acquire(t testing.TB, opts ...PGOption) *PG {
	t.Helper()
	s.once.Do(func() {
		pg := startPostgres(t, opts...)
		if err := pg.connect(); err != nil {
			t.Fatalf("shared postgres: %v", err)
		}
		if err := pg.SnapshotCtx(context.Background()); err != nil {
			t.Fatalf("shared postgres snapshot: %v", err)
		}
		s.pg = pg
	})
	if s.pg == nil {
		t.Fatal("shared postgres failed to start in an earlier test")
	}
	s.pg.Wipe(t)
	return s.pg
}

// SharedK3s returns the suite-wide k3s cluster.
func SharedK3s(t testing.TB) *K3s {
	t.Helper()
	k3sShared.once.Do(func() {
		dir, err := os.MkdirTemp("", "filament-k3s")
		if err != nil {
			t.Fatalf("shared k3s: %v", err)
		}
		k3sShared.cluster = startK3s(t, dir)
	})
	if k3sShared.cluster == nil {
		t.Fatal("shared k3s failed to start in an earlier test")
	}
	k3sShared.cluster.Wipe(t)
	return k3sShared.cluster
}
