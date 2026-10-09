package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
)

var _ filament.SchemaPolicySink = (*Sink)(nil)

// SchemaPolicy exposes the configured reconciliation rules.
func (t *Sink) SchemaPolicy() schema.Policy { return t.evolution }

func schemaPolicy(value string) (schema.Policy, error) {
	if value == "" || value == "disabled" {
		return "", nil
	}
	p := schema.Policy(value)
	return p, p.Validate()
}

func blocked(resource, reason string) error {
	return &schema.BlockError{Resource: resource, Reasons: []string{reason}}
}

func validIdentifier(value string) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= 64 && !strings.ContainsRune(value, 0)
}

// Checked mode uses compiler-owned SQL types, never arbitrary native DDL.
func physicalSchema(resource string, model rowmodel.Schema) (rowmodel.Schema, error) {
	if err := schema.Validate(model); err != nil {
		return rowmodel.Schema{}, blocked(resource, err.Error())
	}
	if !validIdentifier(resource) || len(model.Fields) == 0 {
		return rowmodel.Schema{}, blocked(resource, "complete schema and valid table name required")
	}
	out := rowmodel.Schema{Resource: resource, Engine: engine, PrimaryKey: slices.Clone(model.PrimaryKey)}
	for _, f := range model.Fields {
		typ := columnType(f, false)
		logical := physicalLogical(typ)
		if !validIdentifier(f.Name) || logical == rowmodel.LogicalUnknown || (logical != f.Logical && f.Logical != rowmodel.LogicalTimestampTZ) {
			return out, blocked(resource, fmt.Sprintf("unsupported field %q of type %q", f.Name, f.Logical))
		}
		if f.Logical == rowmodel.LogicalDecimal && (f.Precision < 0 || f.Precision > 65 || f.Scale < 0 || f.Scale > 30 || f.Precision == 0 && f.Scale != 0 || f.Precision > 0 && f.Scale > f.Precision) {
			return out, blocked(resource, "unsupported decimal precision or scale")
		}
		if model.Engine == engine && f.Native != "" && normalizeType(f.Native) != typ {
			return out, blocked(resource, fmt.Sprintf("unsupported native type %q for %q", f.Native, f.Name))
		}
		out.Fields = append(out.Fields, rowmodel.Field{Name: f.Name, Logical: logical, Native: typ, Nullable: f.Nullable && !slices.Contains(out.PrimaryKey, f.Name)})
	}
	return out, nil
}

func normalizeType(typ string) string {
	typ = strings.ToLower(strings.TrimSpace(typ))
	// Integer display widths do not affect storage. tinyint(1) retains bool meaning.
	for _, base := range []string{"smallint", "int", "bigint"} {
		if typ == base+"(6)" || typ == base+"(11)" || typ == base+"(20)" {
			return base
		}
	}
	return typ
}

func physicalLogical(typ string) rowmodel.LogicalType {
	switch typ {
	case "tinyint(1)":
		return rowmodel.LogicalBool
	case "smallint":
		return rowmodel.LogicalInt16
	case "int":
		return rowmodel.LogicalInt32
	case "bigint":
		return rowmodel.LogicalInt64
	case "float":
		return rowmodel.LogicalFloat32
	case "double":
		return rowmodel.LogicalFloat64
	case "longtext":
		return rowmodel.LogicalString
	case "longblob":
		return rowmodel.LogicalBytes
	case "date":
		return rowmodel.LogicalDate
	case "time(6)":
		return rowmodel.LogicalTime
	case "datetime(6)":
		return rowmodel.LogicalTimestamp
	case "json":
		return rowmodel.LogicalJSON
	case "char(36)":
		return rowmodel.LogicalUUID
	}
	if strings.HasPrefix(typ, "decimal(") {
		return rowmodel.LogicalDecimal
	}
	return rowmodel.LogicalUnknown
}

type schemaQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type mysqlSchemaTarget struct {
	conn               *sql.Conn
	database, resource string
}

func (t mysqlSchemaTarget) qualified() string {
	return quoteIdent(t.database) + "." + quoteIdent(t.resource)
}

func (t mysqlSchemaTarget) lock(ctx context.Context) error {
	_, err := t.conn.ExecContext(ctx, "LOCK TABLES "+t.qualified()+" WRITE")
	return err
}

func (t mysqlSchemaTarget) Inspect(ctx context.Context) (rowmodel.Schema, error) {
	return inspectSchema(ctx, t.conn, t.database, t.resource)
}

func (t mysqlSchemaTarget) AddNullable(ctx context.Context, fields []rowmodel.Field) error {
	clauses := make([]string, len(fields))
	for i, f := range fields {
		clauses[i] = "ADD COLUMN " + quoteIdent(f.Name) + " " + f.Native + " NULL"
	}
	if _, err := t.conn.ExecContext(ctx, "ALTER TABLE "+t.qualified()+" "+strings.Join(clauses, ", ")); err != nil {
		return err
	}
	// ALTER can release explicit table locks. Reacquire before verification.
	return t.lock(ctx)
}

// DDL commits independently. Discard this dedicated connection to release all
// explicit locks even after cancellation or an uncertain ALTER response.
func (t *Sink) reconcileSchema(ctx context.Context, desired rowmodel.Schema, ddl string) error {
	conn, err := t.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		_ = conn.Close()
	}()
	if _, err := conn.ExecContext(ctx, ddl+" ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin"); err != nil {
		return err
	}
	target := mysqlSchemaTarget{conn: conn, database: t.database, resource: desired.Resource}
	if err := target.lock(ctx); err != nil {
		return err
	}
	if err := schema.Reconcile(ctx, target, desired, t.evolution); err != nil {
		return err
	}
	if t.modeFor(desired.Resource) == filament.WriteReplace {
		// Clear only after complete validation, while the explicit lock is held.
		_, err = conn.ExecContext(ctx, "TRUNCATE TABLE "+target.qualified())
	}
	return err
}

func inspectSchema(ctx context.Context, q schemaQuerier, database, resource string) (rowmodel.Schema, error) {
	model := rowmodel.Schema{Resource: resource, Engine: engine}
	var kind, storage string
	if err := q.QueryRowContext(ctx, `SELECT TABLE_TYPE, COALESCE(ENGINE,'') FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?`, database, resource).Scan(&kind, &storage); err != nil {
		return model, err
	}
	if kind != "BASE TABLE" || storage != "InnoDB" {
		return model, blocked(resource, "only ordinary InnoDB tables are supported")
	}
	var unsupported int
	err := q.QueryRowContext(ctx, `SELECT
 (SELECT count(*) FROM information_schema.TRIGGERS WHERE EVENT_OBJECT_SCHEMA=? AND EVENT_OBJECT_TABLE=?) +
 (SELECT count(*) FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND PARTITION_NAME IS NOT NULL) +
 (SELECT count(*) FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND CONSTRAINT_TYPE <> 'PRIMARY KEY') +
 (SELECT count(*) FROM information_schema.REFERENTIAL_CONSTRAINTS WHERE UNIQUE_CONSTRAINT_SCHEMA=? AND REFERENCED_TABLE_NAME=?)`, database, resource, database, resource, database, resource, database, resource).Scan(&unsupported)
	if err != nil {
		return model, err
	}
	if unsupported != 0 {
		return model, blocked(resource, "triggers, partitions and non-primary constraints are unsupported")
	}
	rows, err := q.QueryContext(ctx, `SELECT COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,EXTRA,COLUMN_DEFAULT,COALESCE(COLLATION_NAME,'') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? ORDER BY ORDINAL_POSITION`, database, resource)
	if err != nil {
		return model, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var f rowmodel.Field
		var nullable, extra, collation string
		var def sql.NullString
		if err := rows.Scan(&f.Name, &f.Native, &nullable, &extra, &def, &collation); err != nil {
			return model, err
		}
		f.Native = normalizeType(f.Native)
		f.Logical, f.Nullable = physicalLogical(f.Native), nullable == "YES"
		if extra != "" || def.Valid || f.Logical == rowmodel.LogicalUnknown || collation != "" && collation != "utf8mb4_bin" {
			return model, blocked(resource, fmt.Sprintf("column %q has unsupported type, default, generation or collation", f.Name))
		}
		model.Fields = append(model.Fields, f)
	}
	if err := rows.Err(); err != nil {
		return model, err
	}
	_ = rows.Close()
	keys, err := q.QueryContext(ctx, `SELECT COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND CONSTRAINT_NAME='PRIMARY' ORDER BY ORDINAL_POSITION`, database, resource)
	if err != nil {
		return model, err
	}
	defer func() { _ = keys.Close() }()
	for keys.Next() {
		var name string
		if err := keys.Scan(&name); err != nil {
			return model, err
		}
		model.PrimaryKey = append(model.PrimaryKey, name)
	}
	return model, keys.Err()
}

func (t *Sink) validateTable(ctx context.Context, tx *sql.Tx, tbl *table, b *arrowbatch.Batch) error {
	if t.evolution == "" {
		return nil
	}
	if !tbl.input.Equal(b.Rows().Schema()) {
		return blocked(b.Resource, "batch layout differs from prepared schema")
	}
	// Referencing the table acquires a metadata lock held until transaction end.
	rows, err := tx.QueryContext(ctx, "SELECT * FROM "+tbl.qualified+" LIMIT 0") //nolint:gosec // qualified contains only quoteIdent-escaped identifiers
	if err != nil {
		var server *mysql.MySQLError
		if errors.As(err, &server) && server.Number == 1146 {
			return blocked(b.Resource, "prepared destination table no longer exists")
		}
		return err
	}
	_ = rows.Close()
	actual, err := inspectSchema(ctx, tx, t.database, b.Resource)
	if err != nil {
		return err
	}
	diff, err := schema.CompareNamed(actual, tbl.model)
	if err != nil {
		return err
	}
	schema.Observe(ctx, "sink", b.Resource, schema.Strict, diff)
	return schema.Strict.Check(b.Resource, diff)
}
