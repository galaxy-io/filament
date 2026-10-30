package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
)

func schemaPolicy(value string) (schema.Policy, error) {
	if value == "" || value == "disabled" {
		return "", nil
	}
	p := schema.Policy(value)
	return p, p.Validate()
}

// SchemaPolicy supplies the rules used for live message-schema negotiation.
func (t *Sink) SchemaPolicy() schema.Policy { return t.evolution }

func blocked(resource, reason string) error {
	return &schema.BlockError{Resource: resource, Reasons: []string{reason}}
}

func validIdentifier(s string) bool {
	return s != "" && len(s) <= 63 && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

// physicalSchema uses only compiler-owned SQL types. Unsupported native types
// block rather than silently losing precision, array shape, or domain semantics.
func physicalSchema(resource string, source rowmodel.Schema) (rowmodel.Schema, error) {
	if err := schema.Validate(source); err != nil {
		return rowmodel.Schema{}, blocked(resource, err.Error())
	}
	if !validIdentifier(resource) || len(source.Fields) == 0 {
		return rowmodel.Schema{}, blocked(resource, "a complete schema and valid table name are required")
	}
	out := rowmodel.Schema{Resource: resource, Engine: engine, PrimaryKey: slices.Clone(source.PrimaryKey)}
	for _, f := range source.Fields {
		typ := columnType(f, false)
		if !validIdentifier(f.Name) || physicalLogical(typ) != f.Logical || f.Logical == rowmodel.LogicalUnknown {
			return out, blocked(resource, fmt.Sprintf("unsupported field %q of type %q", f.Name, f.Logical))
		}
		if f.Logical == rowmodel.LogicalDecimal && (f.Precision < 0 || f.Precision > 1000 || f.Scale < -1000 || f.Scale > 1000 || f.Precision == 0 && f.Scale != 0) {
			return out, blocked(resource, "unsupported decimal precision or scale")
		}
		if source.Engine == engine && f.Native != "" && normalizeType(f.Native) != normalizeType(typ) {
			return out, blocked(resource, fmt.Sprintf("unsupported native type %q for %q", f.Native, f.Name))
		}
		out.Fields = append(out.Fields, rowmodel.Field{Name: f.Name, Logical: f.Logical, Native: normalizeType(typ), Nullable: f.Nullable && !slices.Contains(out.PrimaryKey, f.Name)})
	}
	return out, nil
}

func normalizeType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if !strings.Contains(s, "(") {
		return baseType(s)
	}
	if strings.HasPrefix(s, "decimal(") {
		s = "numeric(" + strings.TrimPrefix(s, "decimal(")
	}
	return strings.ReplaceAll(s, " ", "")
}

func physicalLogical(native string) rowmodel.LogicalType {
	switch baseType(native) {
	case "boolean":
		return rowmodel.LogicalBool
	case "smallint":
		return rowmodel.LogicalInt16
	case "integer":
		return rowmodel.LogicalInt32
	case "bigint":
		return rowmodel.LogicalInt64
	case "real":
		return rowmodel.LogicalFloat32
	case "double precision":
		return rowmodel.LogicalFloat64
	case "numeric":
		return rowmodel.LogicalDecimal
	case "text":
		return rowmodel.LogicalString
	case "bytea":
		return rowmodel.LogicalBytes
	case "date":
		return rowmodel.LogicalDate
	case "time":
		return rowmodel.LogicalTime
	case "timestamp":
		return rowmodel.LogicalTimestamp
	case "timestamptz":
		return rowmodel.LogicalTimestampTZ
	case "jsonb":
		return rowmodel.LogicalJSON
	case "uuid":
		return rowmodel.LogicalUUID
	default:
		return rowmodel.LogicalUnknown
	}
}

// inspectSchema is called with a table lock held through the enclosing write or
// DDL transaction. Inspection alone is never permission for a later write.
func inspectSchema(ctx context.Context, tx pgx.Tx, namespace, resource string) (rowmodel.Schema, error) {
	model := rowmodel.Schema{Resource: resource, Engine: engine}
	var oid uint32
	var unsupported bool
	err := tx.QueryRow(ctx, `SELECT c.oid,
 c.relkind <> 'r' OR c.relispartition OR c.relrowsecurity OR c.relforcerowsecurity
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_inherits i WHERE i.inhrelid=c.oid OR i.inhparent=c.oid)
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_trigger t WHERE t.tgrelid=c.oid AND NOT t.tgisinternal AND t.tgenabled<>'D')
 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname=$1 AND c.relname=$2`, namespace, resource).Scan(&oid, &unsupported)
	if err != nil {
		return model, err
	}
	if unsupported {
		return model, blocked(resource, "partitioning, inheritance, row security, triggers, or table kind unsupported")
	}
	rows, err := tx.Query(ctx, `SELECT a.attname, pg_catalog.format_type(a.atttypid,a.atttypmod), NOT a.attnotnull,
 n.nspname<>'pg_catalog' OR a.attidentity<>'' OR a.attgenerated<>'' OR a.attcollation<>t.typcollation
 FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_type t ON t.oid=a.atttypid
 JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace
 WHERE a.attrelid=$1 AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum`, oid)
	if err != nil {
		return model, err
	}
	defer rows.Close()
	for rows.Next() {
		var f rowmodel.Field
		if err := rows.Scan(&f.Name, &f.Native, &f.Nullable, &unsupported); err != nil {
			return model, err
		}
		if unsupported {
			return model, blocked(resource, fmt.Sprintf("column %q uses unsupported identity, generation, type, or collation", f.Name))
		}
		f.Native = normalizeType(f.Native)
		f.Logical = physicalLogical(f.Native)
		model.Fields = append(model.Fields, f)
	}
	if err := rows.Err(); err != nil {
		return model, err
	}
	rows.Close()
	err = tx.QueryRow(ctx, `SELECT ARRAY(
 SELECT a.attname::text FROM pg_catalog.pg_constraint c
 CROSS JOIN LATERAL unnest(c.conkey) WITH ORDINALITY k(num,ord)
 JOIN pg_catalog.pg_attribute a ON a.attrelid=c.conrelid AND a.attnum=k.num
 WHERE c.conrelid=$1 AND c.contype='p' ORDER BY k.ord)`, oid).Scan(&model.PrimaryKey)
	return model, err
}

func checkSchema(actual, desired rowmodel.Schema, policy schema.Policy) (schema.Diff, error) {
	diff, err := schema.CompareNamed(actual, desired)
	if err == nil {
		err = policy.Check(desired.Resource, diff)
	}
	return diff, err
}

// reconcileSchema converges from the actual table on every retry. Additions and
// replace-mode truncation share one transaction, after the entire diff is allowed.
// There is no separately persisted migration phase that can disagree with DDL.
func (t *Sink) reconcileSchema(ctx context.Context, desired rowmodel.Schema, createSQL string) error {
	var tx pgx.Tx
	own := t.session.active == nil
	if own {
		var err error
		tx, err = t.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	} else {
		tx = t.session.active.tx
	}
	qualified := pgx.Identifier{t.schema, desired.Resource}.Sanitize()
	// Serialize cooperating creators as PostgreSQL's IF NOT EXISTS alone does
	// not resolve concurrent catalog insertion. Table locks cover external DDL.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", qualified); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, createSQL); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "LOCK TABLE ONLY "+qualified+" IN ACCESS EXCLUSIVE MODE"); err != nil {
		return err
	}
	target := postgresSchemaTarget{tx: tx, namespace: t.schema, resource: desired.Resource}
	if err := schema.Reconcile(ctx, target, desired, t.evolution); err != nil {
		return err
	}

	if t.modeFor(desired.Resource) == filament.WriteReplace {
		if _, err := tx.Exec(ctx, "TRUNCATE "+qualified); err != nil {
			return err
		}
	}
	if own {
		return tx.Commit(ctx)
	}
	return nil
}

func (t *Sink) validateTable(ctx context.Context, tx pgx.Tx, tbl *table) error {
	if t.evolution == "" {
		return nil
	}
	if _, err := tx.Exec(ctx, "LOCK TABLE ONLY "+tbl.qualified+" IN ROW EXCLUSIVE MODE"); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "42P01" || pgErr.Code == "3F000") {
			return blocked(tbl.model.Resource, "prepared destination table no longer exists")
		}
		return err
	}
	actual, err := inspectSchema(ctx, tx, t.schema, tbl.model.Resource)
	if err != nil {
		return err
	}
	diff, err := checkSchema(actual, tbl.model, schema.Strict)
	schema.Observe(ctx, "sink", tbl.model.Resource, schema.Strict, diff)
	return err
}

type postgresSchemaTarget struct {
	tx                  pgx.Tx
	namespace, resource string
}

func (t postgresSchemaTarget) Inspect(ctx context.Context) (rowmodel.Schema, error) {
	return inspectSchema(ctx, t.tx, t.namespace, t.resource)
}

func (t postgresSchemaTarget) AddNullable(ctx context.Context, fields []rowmodel.Field) error {
	clauses := make([]string, len(fields))
	for i, f := range fields {
		clauses[i] = "ADD COLUMN " + pgx.Identifier{f.Name}.Sanitize() + " " + f.Native
	}
	_, err := t.tx.Exec(ctx, "ALTER TABLE "+pgx.Identifier{t.namespace, t.resource}.Sanitize()+" "+strings.Join(clauses, ", "))
	return err
}
