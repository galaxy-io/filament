//go:build integration || e2e

package testcontainers

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	mysqlDatabase = "test"
	mysqlUsername = "test"
	mysqlPassword = "test"
	mysqlRootPass = "root"
)

// MySQL is an ephemeral MySQL server configured for both table reads/writes
// and row-based GTID replication.
type MySQL struct {
	Container tc.Container
	Host      string
	Port      string
	Database  string
	Username  string
	Password  string
	DSN       string
	RootDSN   string
	DB        *sql.DB
	RootDB    *sql.DB
}

// MySQLContainer starts a MySQL 8.4 server with row binlogs and GTIDs enabled,
// grants the test user replication privileges, and registers cleanup.
func MySQLContainer(t testing.TB) *MySQL {
	t.Helper()
	ctx := context.Background()
	ctr, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ProviderType: providerType(t),
		ContainerRequest: tc.ContainerRequest{
			Image: Image(t, "MYSQL_IMAGE"),
			Env: map[string]string{
				"MYSQL_DATABASE":      mysqlDatabase,
				"MYSQL_USER":          mysqlUsername,
				"MYSQL_PASSWORD":      mysqlPassword,
				"MYSQL_ROOT_PASSWORD": mysqlRootPass,
			},
			Cmd: []string{
				"--server-id=1",
				"--log-bin=mysql-bin",
				"--binlog-format=ROW",
				"--binlog-row-image=FULL",
				"--gtid-mode=ON",
				"--enforce-gtid-consistency=ON",
				"--local-infile=ON",
			},
			ExposedPorts: []string{"3306/tcp"},
			WaitingFor: wait.ForListeningPort("3306/tcp").
				WithStartupTimeout(3 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start mysql container: %v", err)
	}
	cleanupContainer(t, "mysql", ctr)

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("mysql host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "3306/tcp")
	if err != nil {
		t.Fatalf("mysql port: %v", err)
	}
	params := "parseTime=true&loc=UTC&multiStatements=true&allowNativePasswords=true"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s", mysqlUsername, mysqlPassword, host, port.Port(), mysqlDatabase, params)
	rootDSN := fmt.Sprintf("root:%s@tcp(%s:%s)/?%s", mysqlRootPass, host, port.Port(), params)

	rootDB, err := sql.Open("mysql", rootDSN)
	if err != nil {
		t.Fatalf("open mysql root connection: %v", err)
	}
	t.Cleanup(func() { _ = rootDB.Close() })
	deadline := time.Now().Add(2 * time.Minute)
	for {
		err = rootDB.PingContext(ctx)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("mysql readiness: %v", err)
		}
		time.Sleep(250 * time.Millisecond)
	}
	if _, err := rootDB.ExecContext(ctx, "GRANT ALL PRIVILEGES ON *.* TO 'test'@'%'"); err != nil {
		t.Fatalf("grant mysql test privileges: %v", err)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open mysql connection: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping mysql: %v", err)
	}

	return &MySQL{
		Container: ctr,
		Host:      host,
		Port:      port.Port(),
		Database:  mysqlDatabase,
		Username:  mysqlUsername,
		Password:  mysqlPassword,
		DSN:       dsn,
		RootDSN:   rootDSN,
		DB:        db,
		RootDB:    rootDB,
	}
}
