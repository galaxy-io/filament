package zerobus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/databricks/internal/connection"
	"github.com/galaxy-io/filament/rowmodel"
)

// tableCreator creates the destination Delta table when create_table is set.
type tableCreator interface {
	CreateTable(ctx context.Context, warehouseID, ddl string) error
	Close() error
}

// executorFactory builds a tableCreator from resolved connection settings.
type executorFactory func(connection.Resolved) (tableCreator, error)

// createTable builds and executes the CREATE TABLE IF NOT EXISTS for a resource.
// Zerobus binds the table name into the stream authorization at creation time,
// so the table (and the service principal's grant) must exist before the stream
// opens — hence this runs inside EnsureSchema, ahead of OpenStream.
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
	ddl, err := createTableDDL(s.catalog, s.schema, resource, schema)
	if err != nil {
		return fmt.Errorf("databrickszerobus sink: %w", err)
	}
	if err := s.exec.CreateTable(ctx, s.resolved.WarehouseID, ddl); err != nil {
		return fmt.Errorf("databrickszerobus sink: create table %s: %w", s.qualify(resource), err)
	}
	return nil
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

// newStatementExecutor builds a tableCreator backed by the Statement Execution
// API, authenticating as the service principal via OAuth client credentials.
func newStatementExecutor(r connection.Resolved) (tableCreator, error) {
	base := strings.TrimRight(r.WorkspaceURL, "/")
	cc := &clientcredentials.Config{
		ClientID:     r.ClientID,
		ClientSecret: r.ClientSecret,
		TokenURL:     base + "/oidc/v1/token",
		Scopes:       []string{"all-apis"},
		AuthStyle:    oauth2.AuthStyleInParams,
	}
	httpClient := cc.Client(context.Background())
	httpClient.Timeout = 60 * time.Second
	return &statementExecutor{workspaceURL: base, client: httpClient}, nil
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

// CreateTable submits the DDL through the Statement Execution API and waits for
// the statement to reach a terminal state before returning.
func (e *statementExecutor) CreateTable(ctx context.Context, warehouseID, ddl string) error {
	if strings.TrimSpace(warehouseID) == "" {
		return fmt.Errorf("warehouse_id is required to create tables")
	}
	payload, err := json.Marshal(map[string]any{
		"warehouse_id": warehouseID,
		"statement":    ddl,
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
