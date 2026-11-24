//go:build integration || e2e

package testcontainers

import (
	"context"
	"database/sql"
	"net"
	"net/url"
	"testing"
	"time"

	_ "github.com/microsoft/go-mssqldb" // registers the "sqlserver" driver
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	mssqlDatabase = "test"
	mssqlUsername = "sa"
	mssqlPassword = "Filament_Test1!"
)

// MSSQL is an ephemeral SQL Server (Developer edition) with a "test" database.
// The image is linux/amd64 only, so on Apple silicon it runs under emulation:
// fine for relative comparisons, not for absolute throughput numbers.
type MSSQL struct {
	Container tc.Container
	Host      string
	Port      string
	Database  string
	Username  string
	Password  string
	// DSN is a go-mssqldb URL for the test database with encryption disabled.
	DSN string
	DB  *sql.DB
}

// MSSQLContainer starts SQL Server, creates the test database with snapshot
// isolation allowed, and registers cleanup.
func MSSQLContainer(t testing.TB) *MSSQL {
	t.Helper()
	ctx := context.Background()
	ctr, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ProviderType: providerType(t),
		ContainerRequest: tc.ContainerRequest{
			Image:         Image(t, "MSSQL_IMAGE"),
			ImagePlatform: "linux/amd64",
			Env: map[string]string{
				"ACCEPT_EULA":       "Y",
				"MSSQL_PID":         "Developer",
				"MSSQL_SA_PASSWORD": mssqlPassword,
				// SQL Agent runs the CDC capture and cleanup jobs.
				"MSSQL_AGENT_ENABLED": "true",
			},
			ExposedPorts: []string{"1433/tcp"},
			WaitingFor: wait.ForLog("SQL Server is now ready for client connections").
				WithStartupTimeout(5 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start mssql container: %v", err)
	}
	cleanupContainer(t, "mssql", ctr)

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("mssql host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "1433/tcp")
	if err != nil {
		t.Fatalf("mssql port: %v", err)
	}

	// A connection timeout bounds the login exchange too: a server that
	// accepts the connection but stalls in prelogin fails the attempt
	// instead of blocking it. The test database's DSN sets one as well.
	master, err := sql.Open("sqlserver", MSSQLDSN(host, port.Port(), "master", url.Values{"connection timeout": {"10"}}))
	if err != nil {
		t.Fatalf("open mssql master connection: %v", err)
	}
	defer func() { _ = master.Close() }()
	// The ready log line precedes the login path accepting connections.
	deadline := time.Now().Add(2 * time.Minute)
	for {
		attempt, cancel := context.WithTimeout(ctx, 15*time.Second)
		err = master.PingContext(attempt)
		cancel()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("mssql readiness: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	setup, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	for _, stmt := range []string{
		"CREATE DATABASE " + mssqlDatabase,
		"ALTER DATABASE " + mssqlDatabase + " SET ALLOW_SNAPSHOT_ISOLATION ON",
		"ALTER DATABASE " + mssqlDatabase + " SET RECOVERY SIMPLE",
	} {
		if _, err := master.ExecContext(setup, stmt); err != nil {
			t.Fatalf("mssql setup %q: %v", stmt, err)
		}
	}

	dsn := MSSQLDSN(host, port.Port(), mssqlDatabase, url.Values{"connection timeout": {"10"}})
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatalf("open mssql connection: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(setup); err != nil {
		t.Fatalf("ping mssql: %v", err)
	}

	return &MSSQL{
		Container: ctr,
		Host:      host,
		Port:      port.Port(),
		Database:  mssqlDatabase,
		Username:  mssqlUsername,
		Password:  mssqlPassword,
		DSN:       dsn,
		DB:        db,
	}
}

// MSSQLDSN builds a go-mssqldb URL for the test server's sa login with
// encryption disabled. extra adds or overrides query parameters, such as
// "packet size".
func MSSQLDSN(host, port, database string, extra url.Values) string {
	q := url.Values{"database": {database}, "encrypt": {"disable"}}
	for k, v := range extra {
		q[k] = v
	}
	u := url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(mssqlUsername, mssqlPassword),
		Host:     net.JoinHostPort(host, port),
		RawQuery: q.Encode(),
	}
	return u.String()
}
