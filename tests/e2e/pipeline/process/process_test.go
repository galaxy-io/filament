//go:build e2e

package process_test

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/api/ingestion/v1/ingestionv1connect"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	gxtc "github.com/galaxy-io/filament/tests/testcontainers"
)

const processTenantID = "00000000-0000-0000-0000-000000000000"

// TestPipelineAcrossRealProcesses is the black-box deployment boundary: a
// compiled API server and control-plane communicate only through containerized
// Postgres and NATS. Runs are deliberately submitted while the control-plane is
// stopped, then consumed after restart through its durable NATS subscription.
func TestPipelineAcrossRealProcesses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	persistence := gxtc.Postgres(t, gxtc.WithDatabase("filament"))
	source := gxtc.Postgres(t)
	destination := gxtc.Postgres(t)
	nats := gxtc.NATSContainer(t)
	if _, err := source.Pool().Exec(ctx, `
		CREATE TABLE process_rows (id bigint PRIMARY KEY, name text NOT NULL, updated_at timestamptz NOT NULL);
		INSERT INTO process_rows VALUES
			(1, 'Ada', '2026-08-01 12:34:56.123456+00'),
			(2, 'Grace', '2026-08-02 01:02:03.000004+00');
	`); err != nil {
		t.Fatal(err)
	}

	serverBinary, controlBinary := buildServiceBinaries(t, ctx)
	t.Log("built server and control-plane binaries")
	serverAddr := freeAddress(t)
	controlAddr := freeAddress(t)
	commonEnv := []string{
		"PERSISTENCE_PROVIDER=postgres",
		"PERSISTENCE_DSN=" + persistence.DSN(),
		"NATS_URL=" + nats.URL,
		"NATS_STREAM=FILAMENT_PROCESS_E2E",
		"NATS_SUBJECTS=ingestion.v1.>",
		"ENCRYPTION_KEY=MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=",
		"ENCRYPTION_KEY_ID=process-e2e",
		"DEFAULT_TENANT_ID=" + processTenantID,
		"LOG_LEVEL=DEBUG",
	}

	runCommand(t, ctx, serverBinary, []string{"-migrate"}, commonEnv)
	t.Log("migrated persistence database")
	server := startManagedProcess(t, serverBinary, nil, append(commonEnv, "SERVER_ADDR="+serverAddr))
	waitForHealth(t, ctx, server, "http://"+serverAddr+"/readyz")
	t.Log("server is ready")

	// Start once before any requests so JetStream creates the durable dispatch
	// consumer, then stop it. The following submission must remain queued.
	control := startManagedProcess(t, controlBinary, nil, append(commonEnv,
		"DISPATCH_MODE=inproc", "HEALTH_ADDR="+controlAddr,
	))
	waitForHealth(t, ctx, control, "http://"+controlAddr+"/readyz")
	control.stop(t)
	t.Log("primed durable control-plane consumers and stopped control-plane")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	api := ingestionv1connect.NewIngestionServiceClient(httpClient, "http://"+serverAddr)
	sourceConnection := createRemoteConnection(t, ctx, api, &ingestionv1.CreateConnectionRequest{
		Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, Name: "process-source", Connector: "postgres",
		Config: testutil.MustStruct(t, map[string]any{"dsn": source.DSN()}),
	})
	sinkConnection := createRemoteConnection(t, ctx, api, &ingestionv1.CreateConnectionRequest{
		Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK, Name: "process-destination", Connector: "postgres",
		Config: testutil.MustStruct(t, map[string]any{"dsn": destination.DSN()}),
	})
	pipeline, err := api.CreatePipeline(ctx, connect.NewRequest(&ingestionv1.CreatePipelineRequest{Name: "process-pipeline"}))
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	pipelineID := pipeline.Msg.GetPipeline().GetId()
	_, err = api.CreatePipelineVersion(ctx, connect.NewRequest(&ingestionv1.CreatePipelineVersionRequest{
		PipelineId: pipelineID,
		Graph: &ingestionv1.PipelineGraph{
			Nodes: []*ingestionv1.PipelineNode{
				{Id: "src", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, ConnectionId: sourceConnection.GetId()},
				{Id: "dst", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK, ConnectionId: sinkConnection.GetId(), Config: testutil.MustStruct(t, map[string]any{"schema": "public", "mode": "typed"})},
			},
			Edges: []*ingestionv1.PipelineEdge{{
				FromNode: "src", ToNode: "dst", Resource: "process_rows",
				ReadMode: ingestionv1.ReadMode_READ_MODE_FULL, WriteMode: ingestionv1.WriteMode_WRITE_MODE_REPLACE,
			}},
		},
	}))
	if err != nil {
		t.Fatalf("CreatePipelineVersion: %v", err)
	}

	firstRun := submitRemoteRun(t, ctx, api, pipelineID, "process-run-1")
	assertRemoteRunStatus(t, ctx, api, firstRun, ingestionv1.RunStatus_RUN_STATUS_REQUESTED)
	t.Logf("submitted queued run %s", firstRun)
	controlAddr = freeAddress(t)
	control = startManagedProcess(t, controlBinary, nil, append(commonEnv,
		"DISPATCH_MODE=inproc", "HEALTH_ADDR="+controlAddr,
	))
	waitForHealth(t, ctx, control, "http://"+controlAddr+"/readyz")
	waitForRemoteRun(t, ctx, api, firstRun)
	t.Logf("completed queued run %s after control-plane restart", firstRun)
	assertProcessRows(t, ctx, destination, []string{
		"1|Ada|2026-08-01 12:34:56.123456",
		"2|Grace|2026-08-02 01:02:03.000004",
	})
	control.stop(t)

	if _, err := source.Pool().Exec(ctx, `
		UPDATE process_rows SET name='Ada Lovelace', updated_at='2026-08-03 00:00:00.000001+00' WHERE id=1;
		DELETE FROM process_rows WHERE id=2;
		INSERT INTO process_rows VALUES (3, 'Linus', '2026-08-03 00:00:00.000002+00');
	`); err != nil {
		t.Fatal(err)
	}
	secondRun := submitRemoteRun(t, ctx, api, pipelineID, "process-run-2")
	assertRemoteRunStatus(t, ctx, api, secondRun, ingestionv1.RunStatus_RUN_STATUS_REQUESTED)
	t.Logf("submitted second queued run %s", secondRun)
	controlAddr = freeAddress(t)
	control = startManagedProcess(t, controlBinary, nil, append(commonEnv,
		"DISPATCH_MODE=inproc", "HEALTH_ADDR="+controlAddr,
	))
	waitForHealth(t, ctx, control, "http://"+controlAddr+"/readyz")
	waitForRemoteRun(t, ctx, api, secondRun)
	t.Logf("completed second queued run %s after control-plane restart", secondRun)
	assertProcessRows(t, ctx, destination, []string{
		"1|Ada Lovelace|2026-08-03 00:00:00.000001",
		"3|Linus|2026-08-03 00:00:00.000002",
	})
}

func buildServiceBinaries(t testing.TB, ctx context.Context) (string, string) {
	t.Helper()
	root := e2eRepositoryRoot(t)
	dir := t.TempDir()
	server := filepath.Join(dir, "server")
	control := filepath.Join(dir, "control-plane")
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	cmd := exec.CommandContext(ctx, goBinary, "build", "-o", dir, "./server", "./control-plane")
	cmd.Dir = filepath.Join(root, "cmd")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build service binaries: %v\n%s", err, output)
	}
	return server, control
}

func e2eRepositoryRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve repository root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

type synchronizedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

type managedProcess struct {
	name string
	cmd  *exec.Cmd
	logs synchronizedBuffer
	done chan struct{}
	mu   sync.Mutex
	err  error
}

func startManagedProcess(t testing.TB, binary string, args, env []string) *managedProcess {
	t.Helper()
	p := &managedProcess{name: filepath.Base(binary), done: make(chan struct{})}
	p.cmd = exec.Command(binary, args...)
	p.cmd.Env = append(os.Environ(), env...)
	p.cmd.Stdout, p.cmd.Stderr = &p.logs, &p.logs
	if err := p.cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", p.name, err)
	}
	go func() {
		err := p.cmd.Wait()
		p.mu.Lock()
		p.err = err
		p.mu.Unlock()
		close(p.done)
	}()
	t.Cleanup(func() {
		p.stop(t)
		if t.Failed() {
			t.Logf("%s logs:\n%s", p.name, p.logs.String())
		}
	})
	return p
}

func (p *managedProcess) stop(t testing.TB) {
	t.Helper()
	select {
	case <-p.done:
		return
	default:
	}
	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Logf("interrupt %s: %v", p.name, err)
	}
	select {
	case <-p.done:
	case <-time.After(10 * time.Second):
		if err := p.cmd.Process.Kill(); err != nil {
			t.Logf("kill %s: %v", p.name, err)
		}
		<-p.done
	}
}

func (p *managedProcess) exitError() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

func runCommand(t testing.TB, ctx context.Context, binary string, args, env []string) {
	t.Helper()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s %v: %v\n%s", filepath.Base(binary), args, err, output)
	}
}

func freeAddress(t testing.TB) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().String()
}

func waitForHealth(t testing.TB, ctx context.Context, process *managedProcess, url string) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client := &http.Client{Timeout: time.Second}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		request, _ := http.NewRequestWithContext(waitCtx, http.MethodGet, url, nil)
		response, err := client.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		select {
		case <-process.done:
			t.Fatalf("%s exited during startup: %v\n%s", process.name, process.exitError(), process.logs.String())
		case <-waitCtx.Done():
			t.Fatalf("wait for %s health: %v\n%s", process.name, waitCtx.Err(), process.logs.String())
		case <-ticker.C:
		}
	}
}

func createRemoteConnection(t testing.TB, ctx context.Context, api ingestionv1connect.IngestionServiceClient, request *ingestionv1.CreateConnectionRequest) *ingestionv1.Connection {
	t.Helper()
	response, err := api.CreateConnection(ctx, connect.NewRequest(request))
	if err != nil {
		t.Fatalf("CreateConnection(%s): %v", request.GetName(), err)
	}
	return response.Msg.GetConnection()
}

func submitRemoteRun(t testing.TB, ctx context.Context, api ingestionv1connect.IngestionServiceClient, pipelineID, token string) string {
	t.Helper()
	response, err := api.RunPipeline(ctx, connect.NewRequest(&ingestionv1.RunPipelineRequest{PipelineId: pipelineID, ClientToken: token}))
	if err != nil {
		t.Fatalf("RunPipeline(%s): %v", token, err)
	}
	if len(response.Msg.GetEdgeRuns()) != 1 || response.Msg.GetEdgeRuns()[0].GetRun().GetId() == "" {
		t.Fatalf("RunPipeline(%s) response = %#v", token, response.Msg)
	}
	return response.Msg.GetEdgeRuns()[0].GetRun().GetId()
}

func getRemoteRun(t testing.TB, ctx context.Context, api ingestionv1connect.IngestionServiceClient, runID string) *ingestionv1.RunSnapshot {
	t.Helper()
	response, err := api.GetRun(ctx, connect.NewRequest(&ingestionv1.GetRunRequest{RunId: runID}))
	if err != nil {
		t.Fatalf("GetRun(%s): %v", runID, err)
	}
	return response.Msg.GetSnapshot()
}

func assertRemoteRunStatus(t testing.TB, ctx context.Context, api ingestionv1connect.IngestionServiceClient, runID string, want ingestionv1.RunStatus) {
	t.Helper()
	if got := getRemoteRun(t, ctx, api, runID).GetRun().GetStatus(); got != want {
		t.Fatalf("run %s status = %s, want %s", runID, got, want)
	}
}

func waitForRemoteRun(t testing.TB, ctx context.Context, api ingestionv1connect.IngestionServiceClient, runID string) {
	t.Helper()
	waitCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot := getRemoteRun(t, waitCtx, api, runID)
		status := snapshot.GetRun().GetStatus()
		switch status {
		case ingestionv1.RunStatus_RUN_STATUS_COMPLETED:
			if snapshot.GetRun().GetRecords() == 0 || len(snapshot.GetResources()) != 1 || snapshot.GetResources()[0].GetStatus() != ingestionv1.RunStatus_RUN_STATUS_COMPLETED {
				t.Fatalf("completed run has incomplete progress: %#v", snapshot)
			}
			return
		case ingestionv1.RunStatus_RUN_STATUS_FAILED, ingestionv1.RunStatus_RUN_STATUS_PARTIAL, ingestionv1.RunStatus_RUN_STATUS_CANCELED:
			t.Fatalf("run %s ended %s: %s", runID, status, snapshot.GetRun().GetError())
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf("wait for run %s: %v (last status %s)", runID, waitCtx.Err(), status)
		case <-ticker.C:
		}
	}
}

func assertProcessRows(t testing.TB, ctx context.Context, destination *gxtc.PG, want []string) {
	t.Helper()
	rows, err := destination.Pool().Query(ctx, `
		SELECT id::text || '|' || name || '|' || to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS.US')
		FROM process_rows ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			t.Fatal(err)
		}
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("destination rows = %v, want %v", got, want)
	}
}
