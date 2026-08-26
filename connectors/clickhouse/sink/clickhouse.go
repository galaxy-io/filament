// Package clickhouse implements a typed ClickHouse sink.
package clickhouse

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/dbconfig"
)

const (
	defaultDatabase          = "default"
	defaultPort              = 9440
	defaultUsername          = "default"
	mergeTreeEngine          = "MergeTree"
	replacingMergeTreeEngine = "ReplacingMergeTree"
	sharedEnginePrefix       = "Shared"
	protocolNative           = "native"
	protocolHTTP             = "http"
)

type connection interface {
	Exec(context.Context, string, ...any) error
	Query(context.Context, string, ...any) (chdriver.Rows, error)
	QueryRow(context.Context, string, ...any) chdriver.Row
	PrepareBatch(context.Context, string, ...chdriver.PrepareBatchOption) (chdriver.Batch, error)
	Ping(context.Context) error
	Close() error
}

// Sink writes typed batches to MergeTree tables and implements key-based upsert
// with ReplacingMergeTree. Snapshot replacement writes into run-scoped staging
// tables and swaps each resource into place only on Commit.
type Sink struct {
	conn     connection
	database string
	run      filament.RunID
	written  atomic.Int64
	policies map[string]filament.WritePolicy
	tables   map[string]*table
}

// modeFor returns the write mode governing one resource: its bound policy, or
// the run-wide policy for runs with no explicit resource list.
func (s *Sink) modeFor(resource string) filament.WriteMode {
	if p, ok := s.policies[resource]; ok {
		return p.Capability.Mode
	}
	return s.policies[""].Capability.Mode
}

type table struct {
	name      string
	qualified string
	writeTo   string
	stage     string
	insertSQL string
	schema    filament.RecordSchema
}

// New returns an unconfigured ClickHouse sink. Open establishes its connection.
func New() *Sink { return &Sink{} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.Schematized       = (*Sink)(nil)
)

// Spec describes the sink's configuration and supported write policies.
func (s *Sink) Spec() filament.SinkSpec {
	fields := dbconfig.VisibleWhen(dbconfig.MethodFields)
	return filament.SinkSpec{
		Name:         "clickhouse",
		DisplayName:  "ClickHouse",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-clickhouse-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-clickhouse-light.svg",
		Description:  "Column-oriented analytics database with typed batch loading and primary-key upserts.",
		Version:      "2",
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			dbconfig.MethodConfigField(),
			dbconfig.DSNConfigField("ClickHouse connection URL"),
			{Name: "host", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "ClickHouse server hostname; copied http:// or https:// endpoints are also accepted"},
			{Name: "port", Type: filament.FieldInt, Default: defaultPort, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "ClickHouse server port (9440 for secure native connections)"},
			{Name: "protocol", Type: filament.FieldEnum, Default: protocolNative, Enum: []filament.EnumOption{{Value: protocolNative, Label: "Native"}, {Value: protocolHTTP, Label: "HTTP"}}, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "ClickHouse wire protocol"},
			{Name: "username", Type: filament.FieldString, Default: defaultUsername, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "ClickHouse username"},
			{Name: "password", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "ClickHouse password"},
			{Name: "database_name", Type: filament.FieldString, Default: defaultDatabase, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Database encoded in the connection; pipeline configuration selects the destination database"},
			{Name: "secure", Type: filament.FieldBool, Default: true, Scope: filament.ScopeConnection, VisibleWhen: fields, Help: "Connect with TLS (required by ClickHouse Cloud)"},
			{Name: "database", Type: filament.FieldString, Default: defaultDatabase, Scope: filament.ScopePipeline, Help: "Destination database. Empty defaults to the normalized source connection name."},
		}},
		SchemaField: "database",
		Capabilities: filament.SinkCapabilities{
			Schematized:        true,
			Upsertable:         true,
			PreferredBatchRows: 10_000,
			WritePolicies: filament.WriteCapabilities(
				filament.IngestionFullReplace,
				filament.IngestionFullAppend,
				filament.IngestionFullUpsert,
				filament.IngestionIncrementalUpsert,
			),
		},
	}
}

// Name identifies this sink implementation.
func (s *Sink) Name() string { return "clickhouse" }

// Validate checks connection syntax without opening a network connection.
func (s *Sink) Validate(cfg filament.Config) error {
	if _, err := connectionOptions(cfg); err != nil {
		return fmt.Errorf("clickhouse sink: connection config: %w", err)
	}
	return nil
}

// TestConnection pings ClickHouse through a short-lived connection without
// creating the configured destination database.
func (s *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	opts, err := connectionOptions(cfg)
	if err != nil {
		return fmt.Errorf("clickhouse sink: connection config: %w", err)
	}
	opts.Auth.Database = defaultDatabase
	conn, err := ch.Open(opts)
	if err != nil {
		return fmt.Errorf("clickhouse sink: open: %w", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("clickhouse sink: ping over %s to %s: %w", opts.Protocol, opts.Addr[0], err)
	}
	return nil
}

// Open connects to ClickHouse and prepares per-run state.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	cfg := filament.NewConfig(run.Sink.Config)
	opts, err := connectionOptions(cfg)
	if err != nil {
		return fmt.Errorf("clickhouse sink: connection config: %w", err)
	}
	s.database = cfg.String("database")
	if s.database == "" {
		s.database = defaultDatabase
	}
	// Connect through the always-present default database so a pipeline can create
	// its destination database on first use.
	opts.Auth.Database = defaultDatabase
	if n := run.Options.SnapshotParallelism; n > 0 {
		opts.MaxOpenConns = max(opts.MaxOpenConns, n)
	}
	conn, err := ch.Open(opts)
	if err != nil {
		return fmt.Errorf("clickhouse sink: open: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return fmt.Errorf("clickhouse sink: ping over %s to %s: %w", opts.Protocol, opts.Addr[0], err)
	}
	if err := conn.Exec(ctx, "CREATE DATABASE IF NOT EXISTS "+quoteIdent(s.database)); err != nil {
		_ = conn.Close()
		return fmt.Errorf("clickhouse sink: create database %q: %w", s.database, err)
	}

	s.conn = conn
	s.run = run.Run
	s.written.Store(0)
	s.policies = run.WritePolicies
	s.tables = map[string]*table{}
	return nil
}

func connectionOptions(cfg filament.Config) (*ch.Options, error) {
	method, err := dbconfig.Method(cfg)
	if err != nil {
		return nil, err
	}
	if method == dbconfig.MethodURL {
		dsn := cfg.Secret(dbconfig.DSNField)
		if strings.TrimSpace(dsn) == "" {
			return nil, fmt.Errorf("dsn is required when connection_method is %q", dbconfig.MethodURL)
		}
		opts, err := ch.ParseDSN(dsn)
		if err != nil {
			return nil, fmt.Errorf("parse dsn: %w", err)
		}
		if len(opts.Addr) == 0 {
			return nil, fmt.Errorf("dsn has no server address")
		}
		return opts, nil
	}

	host, err := normalizeHost(cfg.String("host"))
	if err != nil {
		return nil, err
	}
	port := cfg.Int("port")
	if port == 0 {
		port = defaultPort
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535")
	}

	protocol := cfg.String("protocol")
	if protocol == "" {
		protocol = protocolNative
	}
	var driverProtocol ch.Protocol
	switch protocol {
	case protocolNative:
		driverProtocol = ch.Native
	case protocolHTTP:
		driverProtocol = ch.HTTP
	default:
		return nil, fmt.Errorf("unsupported protocol %q", protocol)
	}

	username := cfg.String("username")
	if username == "" {
		username = defaultUsername
	}
	password := cfg.Secret("password")
	if password == "" {
		return nil, fmt.Errorf("password is required (the configured secret may not have been resolved)")
	}
	opts := &ch.Options{
		Addr:     []string{net.JoinHostPort(host, strconv.Itoa(port))},
		Protocol: driverProtocol,
		Auth: ch.Auth{
			Database: defaultString(cfg.String("database_name"), defaultDatabase),
			Username: username,
			Password: password,
		},
	}
	if driverProtocol == ch.Native {
		opts.Compression = &ch.Compression{Method: ch.CompressionLZ4}
	}
	secure := !cfg.Has("secure") || cfg.Bool("secure")
	if secure {
		opts.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return opts, nil
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func normalizeHost(raw string) (string, error) {
	host := strings.TrimSpace(raw)
	if host == "" {
		return "", fmt.Errorf("host is required")
	}
	if strings.Contains(host, "://") {
		endpoint, err := url.Parse(host)
		if err != nil {
			return "", fmt.Errorf("invalid host endpoint: %w", err)
		}
		if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
			return "", fmt.Errorf("host endpoint scheme must be http or https")
		}
		if endpoint.User != nil || (endpoint.Path != "" && endpoint.Path != "/") || endpoint.RawQuery != "" || endpoint.Fragment != "" {
			return "", fmt.Errorf("host endpoint must not contain credentials, a path, query parameters, or a fragment")
		}
		if endpoint.Port() != "" {
			return "", fmt.Errorf("host endpoint must not contain a port; use the port field")
		}
		host = endpoint.Hostname()
		if host == "" {
			return "", fmt.Errorf("host endpoint has no hostname")
		}
	}
	return strings.Trim(host, "[]"), nil
}

// EnsureSchema creates or evolves one resource table before records arrive.
func (s *Sink) EnsureSchema(ctx context.Context, resource string, schema filament.RecordSchema) error {
	if s.conn == nil {
		return fmt.Errorf("clickhouse sink: ensure schema before open")
	}
	mode := s.modeFor(resource)
	upsert := mode == filament.WriteUpsert
	version := filament.VersionPolicy{}
	if policy, ok := s.policies[resource]; ok {
		version = policy.Version
	} else if policy, ok := s.policies[""]; ok {
		version = policy.Version
	}
	if upsert && version.Strategy == "" {
		version.Strategy = filament.VersionInsertOrder
	}
	stage := ""
	writeTable := resource
	if mode == filament.WriteReplace {
		stage = stageTableName(s.run, resource)
		writeTable = stage
		if err := s.conn.Exec(ctx, "DROP TABLE IF EXISTS "+qualified(s.database, stage)); err != nil {
			return fmt.Errorf("clickhouse sink: drop stale stage for %q: %w", resource, err)
		}
	}

	ddl, idents, err := createTableDDL(s.database, writeTable, schema, upsert, version)
	if err != nil {
		return fmt.Errorf("clickhouse sink: schema for %q: %w", resource, err)
	}
	if err := s.conn.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("clickhouse sink: create table for %q: %w", resource, err)
	}
	if stage == "" {
		if err := s.validateTable(ctx, resource, schema, upsert, version); err != nil {
			return err
		}
		defs, _, err := schemaColumns(schema, upsert, version)
		if err != nil {
			return err
		}
		for _, def := range defs {
			if err := s.conn.Exec(ctx, "ALTER TABLE "+qualified(s.database, resource)+" ADD COLUMN IF NOT EXISTS "+def); err != nil {
				return fmt.Errorf("clickhouse sink: add column on %q: %w", resource, err)
			}
		}
	}

	writeTo := qualified(s.database, writeTable)
	s.tables[resource] = &table{
		name: resource, qualified: qualified(s.database, resource), writeTo: writeTo,
		stage: stage, insertSQL: "INSERT INTO " + writeTo + " (" + strings.Join(idents, ", ") + ")",
		schema: schema,
	}
	return nil
}

func (s *Sink) validateTable(ctx context.Context, resource string, schema filament.RecordSchema, upsert bool, version filament.VersionPolicy) error {
	var engine, engineFull string
	if err := s.conn.QueryRow(ctx,
		"SELECT engine, engine_full FROM system.tables WHERE database = ? AND name = ?", s.database, resource).Scan(&engine, &engineFull); err != nil {
		return fmt.Errorf("clickhouse sink: inspect table %q: %w", resource, err)
	}
	want := mergeTreeEngine
	if upsert {
		want = replacingMergeTreeEngine
	}
	if !matchesTableEngine(engine, want) {
		return fmt.Errorf("clickhouse sink: table %q uses engine %s, need %s for write policy %q", resource, engine, want, s.modeFor(resource))
	}
	if !upsert {
		return nil
	}
	versionField, err := versionField(schema, version)
	if err != nil {
		return fmt.Errorf("clickhouse sink: table %q version policy: %w", resource, err)
	}
	if !matchesReplacingVersion(engineFull, versionField) {
		wantVersion := "insertion order"
		if versionField != "" {
			wantVersion = versionField
		}
		return fmt.Errorf("clickhouse sink: table %q must use %s as its ReplacingMergeTree version", resource, wantVersion)
	}
	rows, err := s.conn.Query(ctx,
		"SELECT name FROM system.columns WHERE database = ? AND table = ? AND is_in_sorting_key ORDER BY name", s.database, resource)
	if err != nil {
		return fmt.Errorf("clickhouse sink: inspect sorting key for %q: %w", resource, err)
	}
	defer func() { _ = rows.Close() }()
	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("clickhouse sink: scan sorting key for %q: %w", resource, err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("clickhouse sink: sorting key for %q: %w", resource, err)
	}
	wantKeys := append([]string(nil), schema.PrimaryKey...)
	sort.Strings(wantKeys)
	if strings.Join(got, "\x00") != strings.Join(wantKeys, "\x00") {
		return fmt.Errorf("clickhouse sink: table %q sorting key %v must exactly match source primary key %v", resource, got, schema.PrimaryKey)
	}
	return nil
}

// Apply validates the requested write policy and inserts one typed batch.
func (s *Sink) Apply(ctx context.Context, batch filament.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if want := s.modeFor(batch.Resource); opts.Policy.Capability.Mode != want {
		return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: apply policy %q does not match resource policy %q", opts.Policy.Capability.Mode, want)
	}
	switch opts.Policy.Capability.Mode {
	case filament.WriteAppend, filament.WriteReplace:
		policy := opts.Policy
		policy.Capability.AcceptsOps = []filament.Operation{filament.OpInsert}
		if err := policy.ValidateRecords(batch.Resource, batch.Records); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: %w", err)
		}
	case filament.WriteUpsert:
		policy := opts.Policy
		policy.Capability.AcceptsOps = []filament.Operation{filament.OpInsert, filament.OpUpdate}
		if err := policy.ValidateRecords(batch.Resource, batch.Records); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: %w", err)
		}
	default:
		return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: write policy %q is not implemented", opts.Policy.Capability.Mode)
	}
	return s.write(ctx, batch)
}

func (s *Sink) write(ctx context.Context, b filament.Batch) (filament.WriteReceipt, error) {
	if s.conn == nil {
		return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: write before open")
	}
	tbl := s.tables[b.Resource]
	if tbl == nil {
		return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: no schema ensured for resource %q", b.Resource)
	}
	batch, err := s.conn.PrepareBatch(ctx, tbl.insertSQL)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: prepare %s seq %d: %w", b.Resource, b.Seq, err)
	}
	defer func() { _ = batch.Close() }()
	var nbytes int64
	for i := range b.Records {
		values, err := decodeRecord(tbl.schema, b.Records[i].Data)
		if err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: decode %s seq %d row %d: %w", b.Resource, b.Seq, i, err)
		}
		if err := batch.Append(values...); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: append %s seq %d row %d: %w", b.Resource, b.Seq, i, err)
		}
		nbytes += int64(len(b.Records[i].Data))
	}
	if err := batch.Send(); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("clickhouse sink: send %s seq %d: %w", b.Resource, b.Seq, err)
	}
	s.written.Add(int64(len(b.Records)))
	crc, _ := filament.CRC32C(b.Records)
	return filament.WriteReceipt{
		URI: fmt.Sprintf("clickhouse://%s.%s", s.database, b.Resource), Bytes: nbytes,
		Rows: len(b.Records), WriteCRC: crc,
	}, nil
}

func matchesReplacingVersion(engineFull, field string) bool {
	compact := strings.NewReplacer(" ", "", "`", "").Replace(engineFull)
	compact = strings.TrimPrefix(compact, sharedEnginePrefix)
	rest, ok := strings.CutPrefix(compact, replacingMergeTreeEngine)
	if !ok {
		return false
	}
	if field != "" {
		return strings.HasPrefix(rest, "("+field+")")
	}
	// ClickHouse may render the parameterless engine with or without (). Reject
	// any non-empty argument so cursor-versioned tables cannot be mistaken for
	// insertion-order tables.
	return !strings.HasPrefix(rest, "(") || strings.HasPrefix(rest, "()")
}

func matchesTableEngine(got, want string) bool {
	return got == want || got == sharedEnginePrefix+want
}

// Commit promotes any snapshot-replacement stages and closes the connection.
func (s *Sink) Commit(ctx context.Context) error {
	defer s.release()
	if s.conn == nil {
		return nil
	}
	for _, tbl := range s.tables {
		if tbl.stage == "" {
			continue
		}
		var exists uint8
		if err := s.conn.QueryRow(ctx, existsTableQuery(tbl.qualified)).Scan(&exists); err != nil {
			return fmt.Errorf("clickhouse sink: inspect destination %q: %w", tbl.name, err)
		}
		if exists == 0 {
			if err := s.conn.Exec(ctx, "RENAME TABLE "+tbl.writeTo+" TO "+tbl.qualified); err != nil {
				return fmt.Errorf("clickhouse sink: promote %q: %w", tbl.name, err)
			}
			continue
		}
		if err := s.conn.Exec(ctx, "EXCHANGE TABLES "+tbl.qualified+" AND "+tbl.writeTo); err != nil {
			return fmt.Errorf("clickhouse sink: replace %q: %w", tbl.name, err)
		}
		if err := s.conn.Exec(ctx, "DROP TABLE "+tbl.writeTo); err != nil {
			return fmt.Errorf("clickhouse sink: drop replaced table %q: %w", tbl.name, err)
		}
	}
	return nil
}

func existsTableQuery(qualifiedTable string) string {
	return "EXISTS TABLE " + qualifiedTable
}

// Abort drops uncommitted snapshot stages and closes the connection.
func (s *Sink) Abort(ctx context.Context) error {
	defer s.release()
	if s.conn == nil {
		return nil
	}
	cleanup := context.WithoutCancel(ctx)
	var first error
	for _, tbl := range s.tables {
		if tbl.stage == "" {
			continue
		}
		if err := s.conn.Exec(cleanup, "DROP TABLE IF EXISTS "+tbl.writeTo); err != nil && first == nil {
			first = fmt.Errorf("clickhouse sink: drop stage for %q: %w", tbl.name, err)
		}
	}
	return first
}

func (s *Sink) release() {
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
}

func stageTableName(run filament.RunID, resource string) string {
	sum := sha256.Sum256([]byte(string(run) + "\x00" + resource))
	return "__filament_stage_" + hex.EncodeToString(sum[:8])
}
