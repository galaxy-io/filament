package zerobus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/databricks/internal/connection"
	"github.com/galaxy-io/filament/rowmodel"
)

// sqlExecutor runs the DDL and grants create_table needs on a SQL warehouse.
type sqlExecutor interface {
	Execute(ctx context.Context, warehouseID, statement string) error
	Close() error
}

// executorFactory builds a sqlExecutor from resolved connection settings.
type executorFactory func(connection.Resolved) (sqlExecutor, error)

// createTable creates the destination schema (once per run) and the resource's
// table, and grants the service principal on both. Zerobus authorizes a stream
// against explicit grants and does not count ownership, so the principal must
// grant itself USE SCHEMA and SELECT, MODIFY even on objects it just created.
// The grants must land before the stream opens, so this runs inside
// EnsureSchema ahead of OpenStream.
//
// A failed CREATE SCHEMA or GRANT does not stop the run: the object may exist
// already with someone else as owner and the grants in place. The failure is
// kept and reported if a later step fails.
func (s *Sink) createTable(ctx context.Context, resource string, schema filament.RecordSchema) error {
	if strings.TrimSpace(s.resolved.WarehouseID) == "" {
		return fmt.Errorf("databrickszerobus sink: create_table requires warehouse_id")
	}
	if s.exec == nil {
		exec, err := s.execFactory()(s.resolved)
		if err != nil {
			return fmt.Errorf("databrickszerobus sink: %w", err)
		}
		s.exec = exec
	}
	principal := s.resolved.ClientID
	if !s.schemaReady {
		s.schemaReady = true
		s.tryExecute(ctx, "create schema", createSchemaSQL(s.catalog, s.schema))
		s.tryExecute(ctx, "grant USE SCHEMA", grantSchemaSQL(s.catalog, s.schema, principal))
	}
	ddl, err := createTableDDL(s.catalog, s.schema, resource, schema)
	if err != nil {
		return fmt.Errorf("databrickszerobus sink: %w", err)
	}
	if err := s.exec.Execute(ctx, s.resolved.WarehouseID, ddl); err != nil {
		return s.withSetupErrors(fmt.Errorf("databrickszerobus sink: create table %s: %w", s.qualify(resource), err))
	}
	s.tryExecute(ctx, "grant SELECT, MODIFY on "+s.qualify(resource), grantTableSQL(s.catalog, s.schema, resource, principal))
	return nil
}

// tryExecute runs a setup statement whose failure is not fatal on its own and
// keeps the error for withSetupErrors.
func (s *Sink) tryExecute(ctx context.Context, step, statement string) {
	if err := s.exec.Execute(ctx, s.resolved.WarehouseID, statement); err != nil {
		s.setupErrs = append(s.setupErrs, fmt.Errorf("%s: %w", step, err))
	}
}

// withSetupErrors adds the setup statements that failed earlier in the run to
// err, since a missing schema or grant is the likely cause of a later failure.
func (s *Sink) withSetupErrors(err error) error {
	if len(s.setupErrs) == 0 {
		return err
	}
	return fmt.Errorf("%w (earlier create_table setup failed: %w)", err, errors.Join(s.setupErrs...))
}

// createSchemaSQL renders the CREATE SCHEMA for the destination schema.
func createSchemaSQL(catalog, schema string) string {
	return "CREATE SCHEMA IF NOT EXISTS " + quoteIdent(catalog) + "." + quoteIdent(schema)
}

// grantSchemaSQL grants USE SCHEMA on the destination schema. A service
// principal is named by its application ID.
func grantSchemaSQL(catalog, schema, principal string) string {
	return "GRANT USE SCHEMA ON SCHEMA " + quoteIdent(catalog) + "." + quoteIdent(schema) + " TO " + quoteIdent(principal)
}

// grantTableSQL grants the privileges a Zerobus stream needs on a table.
func grantTableSQL(catalog, schema, table, principal string) string {
	return "GRANT SELECT, MODIFY ON TABLE " + quoteIdent(catalog) + "." + quoteIdent(schema) + "." + quoteIdent(table) + " TO " + quoteIdent(principal)
}

// createTableDDL renders a Delta CREATE TABLE whose column types match the Arrow
// schema Zerobus validates against (see arrowbatch.Type). Identifiers are
// backtick-quoted.
func createTableDDL(catalog, schema, table string, model rowmodel.Schema) (string, error) {
	if len(model.Fields) == 0 {
		return "", fmt.Errorf("schema for %q has no fields", table)
	}
	cols := make([]string, 0, len(model.Fields))
	for _, f := range model.Fields {
		typ, err := deltaType(f)
		if err != nil {
			return "", fmt.Errorf("column %q: %w", f.Name, err)
		}
		col := quoteIdent(f.Name) + " " + typ
		if !f.Nullable {
			col += " NOT NULL"
		}
		cols = append(cols, col)
	}
	qualified := quoteIdent(catalog) + "." + quoteIdent(schema) + "." + quoteIdent(table)
	return "CREATE TABLE IF NOT EXISTS " + qualified + " (" + strings.Join(cols, ", ") + ") USING DELTA", nil
}

// deltaType maps a portable logical type to the Delta type that matches the
// Arrow storage type sent over Zerobus. Types that travel as utf8 (json, uuid,
// array, unbounded decimal, unknown) land as STRING.
func deltaType(f rowmodel.Field) (string, error) {
	switch f.Logical {
	case rowmodel.LogicalBool:
		return "BOOLEAN", nil
	case rowmodel.LogicalInt16:
		return "SMALLINT", nil
	case rowmodel.LogicalInt32:
		return "INT", nil
	case rowmodel.LogicalInt64:
		return "BIGINT", nil
	case rowmodel.LogicalFloat32:
		return "FLOAT", nil
	case rowmodel.LogicalFloat64:
		return "DOUBLE", nil
	case rowmodel.LogicalDecimal:
		if f.Precision > 0 && f.Precision <= 38 && f.Scale >= 0 && f.Scale <= f.Precision {
			return fmt.Sprintf("DECIMAL(%d,%d)", f.Precision, f.Scale), nil
		}
		return "STRING", nil
	case rowmodel.LogicalBytes:
		return "BINARY", nil
	case rowmodel.LogicalDate:
		return "DATE", nil
	case rowmodel.LogicalTimestamp:
		return "TIMESTAMP_NTZ", nil
	case rowmodel.LogicalTimestampTZ:
		return "TIMESTAMP", nil
	case rowmodel.LogicalString, rowmodel.LogicalJSON, rowmodel.LogicalUUID, rowmodel.LogicalArray, rowmodel.LogicalUnknown:
		return "STRING", nil
	case rowmodel.LogicalTime:
		return "", fmt.Errorf("logical type %q has no Delta equivalent for Zerobus Arrow ingestion; create the table manually", f.Logical)
	default:
		return "STRING", nil
	}
}

// quoteIdent backtick-quotes a Databricks SQL identifier.
func quoteIdent(id string) string {
	return "`" + strings.ReplaceAll(id, "`", "``") + "`"
}

// statementExecutor runs SQL through the Databricks Statement Execution API,
// authenticating as the service principal via OAuth client credentials.
type statementExecutor struct {
	workspaceURL string
	client       *http.Client
}

// newStatementExecutor builds a sqlExecutor backed by the Statement Execution
// API, authenticating as the service principal via OAuth client credentials.
func newStatementExecutor(r connection.Resolved) (sqlExecutor, error) {
	return &statementExecutor{
		workspaceURL: strings.TrimRight(r.WorkspaceURL, "/"),
		client:       connection.HTTPClient(r, 60*time.Second),
	}, nil
}

// Statement polling bounds for statements that are still running after the
// server-side wait_timeout (e.g. a serverless warehouse cold start).
const statementPollTimeout = 5 * time.Minute

// statementPollInterval is a var so tests can shrink it.
var statementPollInterval = 2 * time.Second

// statementResponse is the subset of the Statement Execution API response
// (shared by the submit POST and the poll GET) that we act on.
type statementResponse struct {
	StatementID string `json:"statement_id"`
	Status      struct {
		State string `json:"state"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"status"`
}

// Execute submits the statement through the Statement Execution API and waits
// for it to reach a terminal state before returning.
func (e *statementExecutor) Execute(ctx context.Context, warehouseID, statement string) error {
	if strings.TrimSpace(warehouseID) == "" {
		return fmt.Errorf("warehouse_id is required to run statements")
	}
	payload, err := json.Marshal(map[string]any{
		"warehouse_id": warehouseID,
		"statement":    statement,
		"wait_timeout": "30s",
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.workspaceURL+"/api/2.0/sql/statements", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	out, err := decodeStatement(resp)
	if err != nil {
		return err
	}
	// The statement may still be PENDING/RUNNING past the wait window (a cold
	// serverless warehouse can take minutes). Returning early here would let
	// EnsureSchema open the stream before the table exists, so poll to a
	// terminal state first.
	pollCtx, cancel := context.WithTimeout(ctx, statementPollTimeout)
	defer cancel()
	return e.awaitTerminal(pollCtx, out)
}

// awaitTerminal returns once the statement reaches a terminal state, polling the
// statement endpoint while it is still PENDING/RUNNING.
func (e *statementExecutor) awaitTerminal(ctx context.Context, out statementResponse) error {
	for {
		switch out.Status.State {
		case "SUCCEEDED":
			return nil
		case "FAILED", "CANCELED", "CLOSED":
			return fmt.Errorf("statement %s: %s", out.Status.State, out.Status.Error.Message)
		case "PENDING", "RUNNING":
			// not terminal yet; poll below
		default:
			return fmt.Errorf("statement in unexpected state %q", out.Status.State)
		}
		if out.StatementID == "" {
			return fmt.Errorf("statement is %s but the API returned no statement_id to poll", out.Status.State)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for statement %s to finish: %w", out.StatementID, ctx.Err())
		case <-time.After(statementPollInterval):
		}
		next, err := e.getStatement(ctx, out.StatementID)
		if err != nil {
			return err
		}
		out = next
	}
}

// getStatement fetches the current state of a submitted statement.
func (e *statementExecutor) getStatement(ctx context.Context, id string) (statementResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.workspaceURL+"/api/2.0/sql/statements/"+id, http.NoBody)
	if err != nil {
		return statementResponse{}, err
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return statementResponse{}, err
	}
	return decodeStatement(resp)
}

// decodeStatement reads a statement response, treating a non-2xx status or an
// undecodable body as an error rather than silently passing.
func decodeStatement(resp *http.Response) (statementResponse, error) {
	defer func() { _ = resp.Body.Close() }()
	var out statementResponse
	decErr := json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode/100 != 2 {
		return statementResponse{}, fmt.Errorf("statement execution HTTP %d: %s", resp.StatusCode, out.Status.Error.Message)
	}
	if decErr != nil {
		return statementResponse{}, fmt.Errorf("decode statement response: %w", decErr)
	}
	return out, nil
}

// Close releases executor resources; the HTTP client needs no teardown.
func (e *statementExecutor) Close() error { return nil }
