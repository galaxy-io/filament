package zerobus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newStatementServer serves the submit POST and subsequent poll GETs, returning
// the given sequence of states (one per HTTP call, last repeated if exhausted).
func newStatementServer(t *testing.T, states ...string) (*statementExecutor, *int) {
	t.Helper()
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := states[len(states)-1]
		if calls < len(states) {
			state = states[calls]
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"statement_id":"stmt-1","status":{"state":"` + state + `","error":{"message":"boom"}}}`))
	}))
	t.Cleanup(srv.Close)
	return &statementExecutor{workspaceURL: srv.URL, client: srv.Client()}, &calls
}

// TestCreateTablePollsUntilSucceeded checks CreateTable polls a PENDING/RUNNING
// statement until it reaches SUCCEEDED.
func TestCreateTablePollsUntilSucceeded(t *testing.T) {
	old := statementPollInterval
	statementPollInterval = time.Millisecond
	defer func() { statementPollInterval = old }()

	exec, calls := newStatementServer(t, "PENDING", "RUNNING", "SUCCEEDED")
	if err := exec.CreateTable(context.Background(), "wh-1", "CREATE TABLE t"); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	if *calls != 3 {
		t.Fatalf("expected 3 API calls (submit + 2 polls), got %d", *calls)
	}
}

// TestCreateTableReturnsErrorOnFailedState checks a FAILED statement surfaces as an error.
func TestCreateTableReturnsErrorOnFailedState(t *testing.T) {
	old := statementPollInterval
	statementPollInterval = time.Millisecond
	defer func() { statementPollInterval = old }()

	exec, _ := newStatementServer(t, "RUNNING", "FAILED")
	err := exec.CreateTable(context.Background(), "wh-1", "CREATE TABLE t")
	if err == nil {
		t.Fatal("expected error for FAILED statement")
	}
	if !strings.Contains(err.Error(), "FAILED") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v, want FAILED state and message", err)
	}
}

// TestCreateTableTimesOutWhileRunning checks a never-terminal statement is bounded by the context.
func TestCreateTableTimesOutWhileRunning(t *testing.T) {
	old := statementPollInterval
	statementPollInterval = time.Millisecond
	defer func() { statementPollInterval = old }()

	exec, _ := newStatementServer(t, "RUNNING") // never terminal
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := exec.CreateTable(ctx, "wh-1", "CREATE TABLE t"); err == nil {
		t.Fatal("expected timeout error while statement stays RUNNING")
	}
}

// TestDecodeStatementNon2xxIsError checks a non-2xx response surfaces the HTTP status as an error.
func TestDecodeStatementNon2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"status":{"error":{"message":"denied"}}}`))
	}))
	defer srv.Close()
	exec := &statementExecutor{workspaceURL: srv.URL, client: srv.Client()}
	err := exec.CreateTable(context.Background(), "wh-1", "CREATE TABLE t")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected HTTP 403 error, got %v", err)
	}
}
