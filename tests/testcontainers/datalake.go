//go:build integration

package testcontainers

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	tc "github.com/testcontainers/testcontainers-go"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	_ "github.com/trinodb/trino-go-client/trino" // registers the "trino" sql driver
)

const dataLakeBucket = "warehouse"

// DataLake is an ephemeral Trino → Iceberg-REST → MinIO stack: Trino queries an
// Iceberg catalog served by the in-memory apache/iceberg-rest-fixture, with data
// stored in MinIO. The three containers share a Docker network; everything is
// torn down via t.Cleanup.
type DataLake struct {
	MinIO    *tcminio.MinioContainer
	Catalog  tc.Container // apache/iceberg-rest-fixture
	Trino    tc.Container
	Bucket   string
	TrinoDSN string  // http://test@host:port?catalog=iceberg
	DB       *sql.DB // Trino, "iceberg" catalog over MinIO
}

// tpchTables are the eight standard TPC-H tables.
var tpchTables = []string{"region", "nation", "supplier", "customer", "part", "partsupp", "orders", "lineitem"}

// SeedTPCH materializes TPC-H tables from Trino's built-in tpch connector into the
// Iceberg catalog — real Parquet + metadata land in MinIO — then runs ANALYZE so
// Iceberg carries real table statistics (NDV/min/max), exercising a profiler's
// metastore-stats path. sourceSchema is a tpch schema (e.g. "tiny", "sf1"); tables
// are created under iceberg.<sourceSchema>. With no tables named, all eight seed.
func (l *DataLake) SeedTPCH(t testing.TB, sourceSchema string, tables ...string) {
	t.Helper()
	ctx := context.Background()
	if len(tables) == 0 {
		tables = tpchTables
	}
	if _, err := l.DB.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS iceberg.%s", sourceSchema)); err != nil {
		t.Fatalf("create schema iceberg.%s: %v", sourceSchema, err)
	}
	for _, tbl := range tables {
		ctas := fmt.Sprintf("CREATE TABLE IF NOT EXISTS iceberg.%s.%s AS SELECT * FROM tpch.%s.%s", sourceSchema, tbl, sourceSchema, tbl)
		if _, err := l.DB.ExecContext(ctx, ctas); err != nil {
			t.Fatalf("ctas %s: %v", tbl, err)
		}
		if _, err := l.DB.ExecContext(ctx, fmt.Sprintf("ANALYZE iceberg.%s.%s", sourceSchema, tbl)); err != nil {
			t.Fatalf("analyze %s: %v", tbl, err)
		}
	}
}

// TrinoDataLake starts MinIO, an in-memory Iceberg REST catalog, and Trino wired
// together, then opens a Trino connection against the "iceberg" catalog. Tests
// create namespaces/tables on demand (e.g. CREATE SCHEMA iceberg.demo). Images
// come from MINIO_IMAGE / ICEBERG_REST_IMAGE / TRINO_IMAGE in docker/.env.
func TrinoDataLake(t testing.TB) *DataLake {
	t.Helper()
	ctx := context.Background()

	nw, err := network.New(ctx)
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	t.Cleanup(func() { _ = nw.Remove(ctx) })

	// MinIO, reachable as "minio" on the shared network.
	mc, err := tcminio.Run(ctx, Image(t, "MINIO_IMAGE"),
		tcminio.WithUsername("minioadmin"),
		tcminio.WithPassword("minioadmin"),
		network.WithNetwork([]string{"minio"}, nw),
	)
	if err != nil {
		t.Fatalf("start minio: %v", err)
	}
	t.Cleanup(func() { _ = tc.TerminateContainer(mc) })

	endpoint, err := mc.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("minio endpoint: %v", err)
	}
	s3, err := miniogo.New(endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(mc.Username, mc.Password, ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("minio client: %v", err)
	}
	if err := s3.MakeBucket(ctx, dataLakeBucket, miniogo.MakeBucketOptions{}); err != nil {
		t.Fatalf("make bucket: %v", err)
	}

	// Iceberg REST catalog, reachable as "rest"; in-memory metastore, S3 FileIO at MinIO.
	cat, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        Image(t, "ICEBERG_REST_IMAGE"),
			ExposedPorts: []string{"8181/tcp"},
			Env: map[string]string{
				"CATALOG_WAREHOUSE":              "s3://" + dataLakeBucket + "/",
				"CATALOG_IO__IMPL":               "org.apache.iceberg.aws.s3.S3FileIO",
				"CATALOG_S3_ENDPOINT":            "http://minio:9000",
				"CATALOG_S3_PATH__STYLE__ACCESS": "true",
				"AWS_ACCESS_KEY_ID":              "minioadmin",
				"AWS_SECRET_ACCESS_KEY":          "minioadmin",
				"AWS_REGION":                     "us-east-1",
			},
			Networks:       []string{nw.Name},
			NetworkAliases: map[string][]string{nw.Name: {"rest"}},
			WaitingFor: wait.ForHTTP("/v1/config").WithPort("8181/tcp").
				WithStatusCodeMatcher(func(s int) bool { return s == 200 || s == 400 }).
				WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start iceberg-rest: %v", err)
	}
	t.Cleanup(func() { _ = tc.TerminateContainer(cat) })

	// Trino with an "iceberg" catalog pointed at the REST catalog and MinIO.
	props := strings.Join([]string{
		"connector.name=iceberg",
		"iceberg.catalog.type=rest",
		"iceberg.rest-catalog.uri=http://rest:8181",
		"iceberg.rest-catalog.warehouse=s3://" + dataLakeBucket + "/",
		"fs.native-s3.enabled=true",
		"s3.endpoint=http://minio:9000",
		"s3.region=us-east-1",
		"s3.path-style-access=true",
		"s3.aws-access-key=minioadmin",
		"s3.aws-secret-key=minioadmin",
	}, "\n") + "\n"

	trino, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:          Image(t, "TRINO_IMAGE"),
			ExposedPorts:   []string{"8080/tcp"},
			Networks:       []string{nw.Name},
			NetworkAliases: map[string][]string{nw.Name: {"trino"}},
			Files: []tc.ContainerFile{
				{
					Reader:            strings.NewReader(props),
					ContainerFilePath: "/etc/trino/catalog/iceberg.properties",
					FileMode:          0o644,
				},
				{
					Reader:            strings.NewReader("connector.name=tpch\n"),
					ContainerFilePath: "/etc/trino/catalog/tpch.properties",
					FileMode:          0o644,
				},
			},
			WaitingFor: wait.ForHTTP("/v1/info").WithPort("8080/tcp").
				WithResponseMatcher(func(body io.Reader) bool {
					b, _ := io.ReadAll(body)
					return strings.Contains(string(b), `"starting":false`)
				}).WithStartupTimeout(3 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start trino: %v", err)
	}
	t.Cleanup(func() { _ = tc.TerminateContainer(trino) })

	host, err := trino.Host(ctx)
	if err != nil {
		t.Fatalf("trino host: %v", err)
	}
	port, err := trino.MappedPort(ctx, "8080")
	if err != nil {
		t.Fatalf("trino port: %v", err)
	}
	dsn := fmt.Sprintf("http://test@%s:%s?catalog=iceberg", host, port.Port())
	db, err := sql.Open("trino", dsn)
	if err != nil {
		t.Fatalf("open trino: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("trino ping: %v", err)
	}

	return &DataLake{MinIO: mc, Catalog: cat, Trino: trino, Bucket: dataLakeBucket, TrinoDSN: dsn, DB: db}
}
