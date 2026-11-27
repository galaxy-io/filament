package sqlserver

// Capture instances. SQL Server CDC has no replication slot: the capture
// process (a SQL Agent job, or the built-in scheduler on Azure SQL Database)
// copies committed changes from the log into one change table per capture
// instance, and a cleanup process deletes them by age. Reading holds nothing,
// so any number of pipelines can share an instance, and retention is the
// database's, not this reader's.
//
// The database must already have CDC enabled; that takes sysadmin on SQL
// Server and db_owner on Azure SQL Database, and stays an administrator's
// decision, as wal_level is for PostgreSQL. Per table, the source enables
// capture when manage_capture is on (needs db_owner), the counterpart of a
// managed publication. A table can carry two capture instances at once so its
// captured columns can change without a gap: the reader follows the older one
// up to where the newer one starts, then moves to the newer one.
//
// Only database-scoped catalog views are read, never msdb or Agent tables, so
// the same code serves SQL Server, Azure SQL Managed Instance and Azure SQL
// Database.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// captureInstance is one change table of a source table.
type captureInstance struct {
	name string
	ct   string // [cdc].[<instance>_CT]
	// start is the instance's low watermark: the LSN it began capturing at,
	// raised by cleanup as old changes are deleted. Changes after start are
	// all present.
	start     lsn
	cols      map[string]column // captured columns by name
	commandID bool              // has __$command_id; change tables from older releases lack it
}

// requireCDC fails unless the connected database has CDC enabled.
func (s *Source) requireCDC(ctx context.Context) error {
	var enabled bool
	var name string
	if err := s.db.QueryRowContext(ctx, `SELECT is_cdc_enabled, name FROM sys.databases WHERE database_id = DB_ID()`).Scan(&enabled, &name); err != nil {
		return fmt.Errorf("sqlserver cdc: read database CDC state: %w", err)
	}
	if !enabled {
		return fmt.Errorf("sqlserver cdc: change data capture is not enabled on database %q; run EXEC sys.sp_cdc_enable_db", name)
	}
	return nil
}

// captureInstances lists t's capture instances, oldest first, enabling one
// when the table has none and the source manages capture.
func (s *Source) captureInstances(ctx context.Context, t *table) ([]captureInstance, error) {
	instances, err := s.readCaptureInstances(ctx, t)
	if err != nil || len(instances) > 0 {
		return instances, err
	}
	if !s.manageCapture {
		return nil, fmt.Errorf("sqlserver cdc: table %s has no capture instance; run EXEC sys.sp_cdc_enable_table or set manage_capture", t.qualified)
	}
	var role any // NULL: no gating role
	if s.captureRole != "" {
		role = s.captureRole
	}
	const enable = `EXEC sys.sp_cdc_enable_table @source_schema = @p1, @source_name = @p2, @role_name = @p3, @supports_net_changes = 0`
	if _, err := s.db.ExecContext(ctx, enable, s.schema, t.name, role); err != nil {
		return nil, fmt.Errorf("sqlserver cdc: enable capture on %s: %w", t.qualified, err)
	}
	instances, err = s.readCaptureInstances(ctx, t)
	if err == nil && len(instances) == 0 {
		err = fmt.Errorf("sqlserver cdc: capture on %s was enabled but no capture instance is listed", t.qualified)
	}
	return instances, err
}

func (s *Source) readCaptureInstances(ctx context.Context, t *table) ([]captureInstance, error) {
	const q = `
SELECT ct.capture_instance, ct.start_lsn, ct.object_id, OBJECT_NAME(ct.object_id)
FROM cdc.change_tables ct
WHERE ct.source_object_id = OBJECT_ID(@p1)
ORDER BY ct.start_lsn, ct.create_date`
	rows, err := s.db.QueryContext(ctx, q, t.qualified)
	if err != nil {
		return nil, fmt.Errorf("sqlserver cdc: capture instances of %s: %w", t.qualified, err)
	}
	type ref struct {
		inst     captureInstance
		objectID int
	}
	var refs []ref
	for rows.Next() {
		var r ref
		var start []byte
		var ctName string
		if err := rows.Scan(&r.inst.name, &start, &r.objectID, &ctName); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("sqlserver cdc: scan capture instance: %w", err)
		}
		if r.inst.start, err = lsnFromBytes(start); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("sqlserver cdc: capture instance %q start: %w", r.inst.name, err)
		}
		r.inst.ct = quoteIdent("cdc") + "." + quoteIdent(ctName)
		refs = append(refs, r)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, fmt.Errorf("sqlserver cdc: capture instances of %s: %w", t.qualified, err)
	}
	instances := make([]captureInstance, len(refs))
	for i, r := range refs {
		cols, err := s.objectColumns(ctx, r.objectID)
		if err != nil {
			return nil, fmt.Errorf("sqlserver cdc: columns of %s: %w", r.inst.ct, err)
		}
		r.inst.cols = map[string]column{}
		for _, c := range cols {
			switch {
			case c.name == "__$command_id":
				r.inst.commandID = true
			case !strings.HasPrefix(c.name, "__$"):
				r.inst.cols[c.name] = c
			}
		}
		instances[i] = r.inst
	}
	return instances, nil
}

// objectColumns reads an object's columns with the type detail column.decl
// needs, falling back to the system type under an alias type the login
// cannot see, as tableMeta does. Change tables keep each captured column's type as it was when the
// instance was created, except rowversion, which they store as binary(8).
func (s *Source) objectColumns(ctx context.Context, objectID int) ([]column, error) {
	const q = `
SELECT c.name,
	CASE WHEN t.is_assembly_type = 1 THEN t.name ELSE bt.name END,
	c.max_length, c.precision, c.scale, c.is_nullable
FROM sys.columns c
LEFT JOIN sys.types t ON t.user_type_id = c.user_type_id
LEFT JOIN sys.types bt ON bt.user_type_id = c.system_type_id
WHERE c.object_id = @p1
ORDER BY c.column_id`
	rows, err := s.db.QueryContext(ctx, q, objectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var cols []column
	for rows.Next() {
		var c column
		var typeName sql.NullString
		var maxLength int16
		var precision, scale uint8
		if err := rows.Scan(&c.name, &typeName, &maxLength, &precision, &scale, &c.nullable); err != nil {
			return nil, err
		}
		if !typeName.Valid {
			return nil, fmt.Errorf("column %q has a type this login cannot see; grant it REFERENCES on the type", c.name)
		}
		c.typeName = typeName.String
		c.maxLength, c.precision, c.scale = int(maxLength), int(precision), int(scale)
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// readPlan is the capture instance a cycle reads a table from and the LSN the
// read stops at. While a newer instance does not yet cover the table's
// position, the older one is read up to where the newer one starts.
type readPlan struct {
	inst *captureInstance
	// stop caps the read at the newer instance's start; zero means no cap.
	stop lsn
	// rotating is set while reading an older instance that a newer one
	// replaces.
	rotating bool
	// uncaptured lists the table's columns the older instance does not
	// capture. A change it holds has no value for them, so reading one fails
	// rather than guess.
	uncaptured []string
}

// planRead picks the instance to read changes after from. Instances are
// oldest first. An instance holds every change from its low watermark on, so
// it serves a read whose first change, the one just after from, is at or
// above it. Otherwise cleanup deleted changes the read needs, or they were
// never captured.
func planRead(t *table, instances []captureInstance, from lsn) (readPlan, error) {
	first := from.next()
	newest := &instances[len(instances)-1]
	if newest.start.compare(first) <= 0 {
		return readPlan{inst: newest}, checkCaptured(t, newest, nil)
	}
	for i := len(instances) - 2; i >= 0; i-- {
		if instances[i].start.compare(first) <= 0 {
			plan := readPlan{inst: &instances[i], stop: newest.start, rotating: true}
			for _, f := range t.schema.Fields {
				if _, ok := plan.inst.cols[f.Name]; !ok {
					plan.uncaptured = append(plan.uncaptured, f.Name)
				}
			}
			return plan, checkCaptured(t, plan.inst, newest)
		}
	}
	return readPlan{}, fmt.Errorf("sqlserver cdc: changes to %s after %s are no longer in capture instance %q (its oldest retained change is %s); "+
		"cleanup removed them or capture started later, so the table must be snapshotted again",
		t.qualified, from, newest.name, newest.start)
}

// checkCaptured fails when inst lacks a column of the table's schema, unless
// the newer instance replacing it captures that column.
func checkCaptured(t *table, inst, next *captureInstance) error {
	for _, f := range t.schema.Fields {
		if _, ok := inst.cols[f.Name]; ok {
			continue
		}
		if next != nil {
			if _, ok := next.cols[f.Name]; ok {
				continue
			}
		}
		return fmt.Errorf("sqlserver cdc: capture instance %q of %s does not capture column %q; "+
			"create a second capture instance that does (sys.sp_cdc_enable_table with a new @capture_instance) and the source will move to it",
			inst.name, t.qualified, f.Name)
	}
	return nil
}
