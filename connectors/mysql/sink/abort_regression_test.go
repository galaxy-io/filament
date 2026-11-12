package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
)

type cleanupDriver struct{ calls map[string]int }

func (d *cleanupDriver) Open(string) (driver.Conn, error)             { return &cleanupConn{d}, nil }
func (d *cleanupDriver) Connect(context.Context) (driver.Conn, error) { return d.Open("") }
func (d *cleanupDriver) Driver() driver.Driver                        { return d }

type cleanupConn struct{ d *cleanupDriver }

func (*cleanupConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (*cleanupConn) Close() error              { return nil }
func (*cleanupConn) Begin() (driver.Tx, error) { return nil, errors.New("unexpected transaction") }
func (c *cleanupConn) ExecContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Result, error) {
	c.d.calls[q]++
	if strings.Contains(q, "broken") {
		return nil, errors.New(q)
	}
	return driver.RowsAffected(0), nil
}

func TestAbortAttemptsEveryReplacement(t *testing.T) {
	d := &cleanupDriver{calls: map[string]int{}}
	db := sql.OpenDB(d)
	defer db.Close()
	s := New()
	s.db = db
	s.policies = map[string]filament.WritePolicy{"": filament.IngestionFullReplace.WritePolicy()}
	s.tables = map[string]*table{"broken_a": {qualified: "broken_a"}, "broken_b": {qualified: "broken_b"}, "healthy": {qualified: "healthy"}, "resumable": {qualified: "resumable", resumable: true}}
	err := s.Abort(context.Background())
	if err == nil || !strings.Contains(err.Error(), "broken_a") || !strings.Contains(err.Error(), "broken_b") {
		t.Fatalf("lost cleanup failures: %v", err)
	}
	for _, name := range []string{"broken_a", "broken_b", "healthy"} {
		if d.calls["TRUNCATE "+name] != 1 {
			t.Fatalf("cleanup skipped %s", name)
		}
	}
	if d.calls["TRUNCATE resumable"] != 0 {
		t.Fatal("truncated resumable table")
	}
}
