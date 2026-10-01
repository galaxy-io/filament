package bigquery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	storage "cloud.google.com/go/bigquery/storage/apiv1"
	"cloud.google.com/go/bigquery/storage/apiv1/storagepb"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

type fakeBigQuery struct {
	storagepb.UnimplementedBigQueryWriteServer
	mu                                              sync.Mutex
	tables                                          map[string]json.RawMessage
	jobs                                            map[string]map[string]any
	streams                                         map[string]int64
	ordinals                                        map[string][]int64
	statements                                      []string
	appends, finalizes, commits                     int
	appendLogicalBytes, appendCompressedBytes       int64
	loseAppend, badCount, rejectCommit, failPublish bool
	holdAppend                                      <-chan struct{}
	appendStarted                                   chan struct{}
	ackPermits                                      <-chan struct{}
	requests                                        chan int64
	disconnectAfter                                 int
	rejectAppends                                   bool
	requestOffsets                                  []int64
}

func newFixture(t *testing.T, mode filament.IngestionType, resources ...string) (*Sink, *fakeBigQuery) {
	t.Helper()
	fake := &fakeBigQuery{tables: map[string]json.RawMessage{}, jobs: map[string]map[string]any{}, streams: map[string]int64{}, ordinals: map[string][]int64{}}
	server := httptest.NewServer(http.HandlerFunc(fake.serveHTTP))
	t.Cleanup(server.Close)
	client, err := bigquery.NewClient(context.Background(), "project", option.WithEndpoint(server.URL), option.WithHTTPClient(server.Client()), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	listener := bufconn.Listen(16 << 20)
	grpcServer := grpc.NewServer(grpc.MaxRecvMsgSize(12<<20), grpc.StatsHandler(&appendWireStats{fake: fake}))
	storagepb.RegisterBigQueryWriteServer(grpcServer, fake)
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(grpcServer.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	writer, err := storage.NewBigQueryWriteClient(context.Background(), option.WithGRPCConn(conn))
	if err != nil {
		t.Fatal(err)
	}
	sink := &Sink{client: client, storage: writer, project: "project", dataset: "dataset", location: "US"}
	policies := map[string]filament.WritePolicy{}
	for _, resource := range resources {
		policies[resource] = filament.WritePolicy{Capability: bigQueryWriteCapabilities(mode)[0], Keys: []string{"id"}}
	}
	sink.initialize(context.Background(), filament.RunSpec{Run: "test-run", WritePolicies: policies})
	t.Cleanup(func() { _ = sink.Abort(context.Background()) })
	for _, resource := range resources {
		if err := sink.EnsureSchema(context.Background(), resource, rowmodel.Schema{Fields: []rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}, {Name: "value", Logical: rowmodel.LogicalString, Nullable: true}}, PrimaryKey: []string{"id"}}); err != nil {
			t.Fatal(err)
		}
	}
	return sink, fake
}

func (f *fakeBigQuery) serveHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	parts := strings.Split(r.URL.Path, "/")
	name := parts[len(parts)-1]
	if strings.Contains(r.URL.Path, "/tables") {
		switch r.Method {
		case "GET":
			table, ok := f.tables[name]
			if !ok {
				http.Error(w, `{"error":{"code":404,"message":"missing"}}`, 404)
				return
			}
			_, _ = w.Write(table)
		case "POST":
			var body map[string]json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&body)
			var ref struct {
				Table string `json:"tableId"`
			}
			_ = json.Unmarshal(body["tableReference"], &ref)
			encoded, _ := json.Marshal(body)
			f.tables[ref.Table] = encoded
			_, _ = w.Write(encoded)
		case "PATCH":
			var body map[string]json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&body)
			var table map[string]json.RawMessage
			_ = json.Unmarshal(f.tables[name], &table)
			for key, value := range body {
				table[key] = value
			}
			encoded, _ := json.Marshal(table)
			f.tables[name] = encoded
			_, _ = w.Write(encoded)
		case "DELETE":
			delete(f.tables, name)
			w.WriteHeader(204)
		default:
			http.Error(w, "unexpected metadata request", 500)
		}
		return
	}
	if strings.Contains(r.URL.Path, "/queries/") {
		_ = json.NewEncoder(w).Encode(map[string]any{"jobComplete": true, "jobReference": map[string]any{"projectId": "project", "jobId": name, "location": "US"}})
		return
	}
	if strings.Contains(r.URL.Path, "/jobs") {
		if r.Method == "GET" {
			job, ok := f.jobs[name]
			if !ok {
				http.Error(w, `{"error":{"code":404,"message":"missing"}}`, 404)
				return
			}
			_ = json.NewEncoder(w).Encode(job)
			return
		}
		var job map[string]any
		_ = json.NewDecoder(r.Body).Decode(&job)
		id := job["jobReference"].(map[string]any)["jobId"].(string)
		statement := job["configuration"].(map[string]any)["query"].(map[string]any)["query"].(string)
		f.statements = append(f.statements, statement)
		job["status"] = map[string]any{"state": "DONE"}
		if f.failPublish {
			job["status"] = map[string]any{"state": "DONE", "errorResult": map[string]any{"reason": "invalidQuery", "message": "publication failed"}}
		}
		f.jobs[id] = job
		_ = json.NewEncoder(w).Encode(job)
		return
	}
	http.Error(w, "unexpected request", 500)
}

func (f *fakeBigQuery) CreateWriteStream(_ context.Context, r *storagepb.CreateWriteStreamRequest) (*storagepb.WriteStream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	name := r.Parent + "/streams/pending"
	f.streams[name] = 0
	return &storagepb.WriteStream{Name: name, Type: storagepb.WriteStream_PENDING}, nil
}

func (f *fakeBigQuery) AppendRows(stream storagepb.BigQueryWrite_AppendRowsServer) error {
	if md, ok := metadata.FromIncomingContext(stream.Context()); !ok || len(md.Get("x-goog-request-params")) == 0 {
		return status.Error(codes.InvalidArgument, "missing stream routing metadata")
	}
	sendResponse := stream.Send
	if f.ackPermits != nil {
		// Read requests independently of replying, like a bidi service with
		// delayed acknowledgements. This exposes stop-and-wait clients.
		ctx, cancel := context.WithCancel(stream.Context())
		responses := make(chan *storagepb.AppendRowsResponse, 16)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				var response *storagepb.AppendRowsResponse
				select {
				case response = <-responses:
				case <-ctx.Done():
					return
				}
				select {
				case <-f.ackPermits:
				case <-ctx.Done():
					return
				}
				if err := stream.Send(response); err != nil {
					return
				}
			}
		}()
		defer func() { cancel(); <-done }()
		sendResponse = func(response *storagepb.AppendRowsResponse) error {
			select {
			case responses <- response:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	var schema []byte
	for {
		request, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		f.mu.Lock()
		f.requestOffsets = append(f.requestOffsets, request.Offset.Value)
		f.mu.Unlock()
		if f.requests != nil {
			select {
			case f.requests <- request.Offset.Value:
			default:
			}
		}
		if f.appendStarted != nil {
			select {
			case f.appendStarted <- struct{}{}:
			default:
			}
		}
		if f.holdAppend != nil {
			select {
			case <-f.holdAppend:
			case <-stream.Context().Done():
				return stream.Context().Err()
			}
		}
		rows := request.GetArrowRows()
		if rows.WriterSchema != nil {
			schema = rows.WriterSchema.SerializedSchema
		}
		if len(schema) == 0 {
			return status.Error(codes.InvalidArgument, "missing Arrow schema")
		}
		if f.rejectAppends {
			if err := sendResponse(&storagepb.AppendRowsResponse{Response: &storagepb.AppendRowsResponse_Error{Error: status.New(codes.InvalidArgument, "invalid append schema").Proto()}}); err != nil {
				return err
			}
			continue
		}
		f.mu.Lock()
		f.appends++
		offset := request.Offset.Value
		if offset < f.streams[request.WriteStream] {
			f.mu.Unlock()
			if err := sendResponse(&storagepb.AppendRowsResponse{Response: &storagepb.AppendRowsResponse_Error{Error: status.New(codes.AlreadyExists, "duplicate offset").Proto()}}); err != nil {
				return err
			}
			continue
		}
		if offset != f.streams[request.WriteStream] {
			f.mu.Unlock()
			return status.Error(codes.OutOfRange, "bad offset")
		}
		// Decode the actual payload received over gRPC, including schema IPC.
		reader, err := ipc.NewReader(io.MultiReader(bytes.NewReader(schema), bytes.NewReader(rows.Rows.SerializedRecordBatch)))
		if err != nil {
			f.mu.Unlock()
			return err
		}
		if !reader.Next() {
			err = reader.Err()
			reader.Release()
			f.mu.Unlock()
			return fmt.Errorf("empty IPC: %v", err)
		}
		record := reader.RecordBatch()
		ordinal := record.Column(int(record.NumCols() - 1)).(*array.Int64)
		for i := 0; i < ordinal.Len(); i++ {
			f.ordinals[request.WriteStream] = append(f.ordinals[request.WriteStream], ordinal.Value(i))
		}
		f.streams[request.WriteStream] += record.NumRows()
		reader.Release()
		lose := f.loseAppend || (f.disconnectAfter > 0 && f.appends == f.disconnectAfter)
		f.loseAppend = false
		f.mu.Unlock()
		if lose {
			return status.Error(codes.Unavailable, "response lost after persistence")
		}
		if err := sendResponse(&storagepb.AppendRowsResponse{Response: &storagepb.AppendRowsResponse_AppendResult_{AppendResult: &storagepb.AppendRowsResponse_AppendResult{Offset: wrapperspb.Int64(offset)}}}); err != nil {
			return err
		}
	}
}

func (f *fakeBigQuery) FinalizeWriteStream(_ context.Context, r *storagepb.FinalizeWriteStreamRequest) (*storagepb.FinalizeWriteStreamResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.finalizes++
	rows := f.streams[r.Name]
	if f.badCount {
		rows++
	}
	return &storagepb.FinalizeWriteStreamResponse{RowCount: rows}, nil
}

func (f *fakeBigQuery) BatchCommitWriteStreams(_ context.Context, r *storagepb.BatchCommitWriteStreamsRequest) (*storagepb.BatchCommitWriteStreamsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commits++
	if f.rejectCommit {
		return &storagepb.BatchCommitWriteStreamsResponse{StreamErrors: []*storagepb.StorageError{{ErrorMessage: "rejected"}}}, nil
	}
	return &storagepb.BatchCommitWriteStreamsResponse{CommitTime: timestamppb.Now()}, nil
}

func testBatch(resource string, n int) *arrowbatch.Batch {
	id := array.NewInt64Builder(memory.DefaultAllocator)
	value := array.NewStringBuilder(memory.DefaultAllocator)
	defer id.Release()
	defer value.Release()
	for i := 0; i < n; i++ {
		id.Append(int64(i))
		value.Append(strings.Repeat("x", 80))
	}
	ids, values := id.NewArray(), value.NewArray()
	defer ids.Release()
	defer values.Release()
	record := array.NewRecordBatch(arrow.NewSchema([]arrow.Field{{Name: "id", Type: arrow.PrimitiveTypes.Int64}, {Name: "value", Type: arrow.BinaryTypes.String, Nullable: true}}, nil), []arrow.Array{ids, values}, int64(n))
	batch := arrowbatch.NewBatch(record, arrowbatch.Operations{})
	batch.Resource = resource
	return batch
}

func applyBatch(t *testing.T, sink *Sink, resource string, n int) {
	t.Helper()
	batch := testBatch(resource, n)
	defer batch.Release()
	receipt, err := sink.Apply(context.Background(), batch, filament.ApplyOptions{Policy: sink.policies[resource]})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Rows != n || receipt.WriteCRC != batch.IntegrityCRC() || receipt.EncodedCRC == nil {
		t.Fatalf("invalid receipt: %+v", receipt)
	}
}

func TestManyBatchesUseOnePublicationPerTable(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullUpsert, "one", "two")
	for i := 0; i < 10; i++ {
		applyBatch(t, sink, "one", 4)
		applyBatch(t, sink, "two", 3)
	}
	fake.mu.Lock()
	before := len(fake.statements)
	fake.mu.Unlock()
	if before != 0 {
		t.Fatalf("Apply submitted %d query jobs", before)
	}
	if err := sink.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.statements) != 2 || fake.commits != 2 || fake.finalizes != 2 {
		t.Fatalf("jobs=%d commits=%d finalizes=%d", len(fake.statements), fake.commits, fake.finalizes)
	}
	for _, statement := range fake.statements {
		_, statement, _ = strings.Cut(statement, "\n")
		if !strings.HasPrefix(statement, "MERGE") || !strings.Contains(statement, "QUALIFY ROW_NUMBER()") {
			t.Fatal(statement)
		}
	}
	for name, ordinals := range fake.ordinals {
		for i, ordinal := range ordinals {
			if ordinal != int64(i) {
				t.Fatalf("%s ordinal[%d]=%d", name, i, ordinal)
			}
		}
	}
	if len(fake.tables) != 2 {
		t.Fatalf("staging tables remain: %v", fake.tables)
	}
}

func TestLostAppendResponseRetriesSameOffset(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "one")
	fake.loseAppend = true
	applyBatch(t, sink, "one", 5)
	applyBatch(t, sink, "one", 3)
	if err := sink.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.appends != 3 {
		t.Fatalf("appends=%d, want original/retry/next", fake.appends)
	}
	for _, rows := range fake.streams {
		if rows != 8 {
			t.Fatalf("rows=%d, want 8 without duplicate", rows)
		}
	}
}

func TestStreamValidationFailureDoesNotPublish(t *testing.T) {
	for _, kind := range []string{"row count", "stream error"} {
		t.Run(kind, func(t *testing.T) {
			sink, fake := newFixture(t, filament.IngestionFullAppend, "one")
			fake.badCount = kind == "row count"
			fake.rejectCommit = kind == "stream error"
			applyBatch(t, sink, "one", 5)
			if err := sink.Commit(context.Background()); err == nil {
				t.Fatal("expected stream validation error")
			}
			if len(fake.statements) != 0 {
				t.Fatal("published invalid stream")
			}
		})
	}
}

func TestEmptyReplacementTruncatesAndEmptyAppendDoesNot(t *testing.T) {
	for _, mode := range []filament.IngestionType{filament.IngestionFullReplace, filament.IngestionFullAppend} {
		t.Run(string(mode), func(t *testing.T) {
			sink, fake := newFixture(t, mode, "empty")
			if err := sink.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			if fake.appends != 0 || fake.commits != 0 {
				t.Fatal("created an empty stream")
			}
			if mode == filament.IngestionFullReplace {
				if len(fake.statements) != 1 || !strings.Contains(fake.statements[0], "\nTRUNCATE TABLE ") {
					t.Fatal(fake.statements)
				}
			} else if len(fake.statements) != 0 {
				t.Fatal(fake.statements)
			}
		})
	}
}

func TestAbortCancelsBlockedUploads(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "one")
	fake.holdAppend = make(chan struct{})
	fake.appendStarted = make(chan struct{}, 1)
	applyBatch(t, sink, "one", 5)
	select {
	case <-fake.appendStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("upload never started")
	}
	done := make(chan struct{})
	go func() { _ = sink.Abort(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Abort did not cancel blocked upload")
	}
	if !sink.memory.TryAcquire(queuedBytes) {
		t.Fatal("queued memory leaked")
	}
	sink.memory.Release(queuedBytes)
}

func TestFailedPublicationRetainsStage(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "one")
	fake.failPublish = true
	applyBatch(t, sink, "one", 3)
	if err := sink.Commit(context.Background()); err == nil {
		t.Fatal("expected publication error")
	}
	_ = sink.Abort(context.Background())
	if len(fake.tables) != 2 {
		t.Fatal("stage deleted before publication outcome was safe")
	}
}

func TestPublicationSegmentRecovery(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "one")
	sink.execution = "first-dispatch"
	applyBatch(t, sink, "one", 3)
	sink.seal()
	sink.workers.Wait()
	state := sink.tables["one"]
	if err := sink.publish(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	identity := jobID(sink.run, state.definition.qualified, "publish_v3", 0, 0, sink.execution+"\x00"+string(filament.WriteAppend))
	marker, _, _ := strings.Cut(fake.statements[0], "\n")
	// Recovery with identical data/checkpoints can use another staging table.
	if err := sink.runQuery(context.Background(), marker+"\nINSERT FROM ANOTHER STAGE", identity); err != nil {
		t.Fatal(err)
	}
	if len(fake.statements) != 1 {
		t.Fatal("recovery submitted another job")
	}
	if err := sink.runQuery(context.Background(), "-- filament-segment:different\nINSERT NEW DATA", identity); err == nil {
		t.Fatal("changed segment silently acknowledged")
	}
	// A continuation of the same run is a distinct publication even if rows match.
	sink.execution = "resume-dispatch"
	if err := sink.publish(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if len(fake.statements) != 2 {
		t.Fatal("continuation reused previous publication")
	}
}

func TestPauseDrainsUploads(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionIncrementalUpsert, "one")
	extraction, pause := context.WithCancel(context.Background())
	sink.cancel()
	// initialize is the production owner of the detached upload context.
	policies := sink.policies
	sink.initialize(extraction, filament.RunSpec{Run: "pause-run", WritePolicies: policies})
	if err := sink.EnsureSchema(context.Background(), "one", rowmodel.Schema{Fields: []rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}, {Name: "value", Logical: rowmodel.LogicalString, Nullable: true}}, PrimaryKey: []string{"id"}}); err != nil {
		t.Fatal(err)
	}
	held := make(chan struct{})
	fake.holdAppend = held
	fake.appendStarted = make(chan struct{}, 1)
	applyBatch(t, sink, "one", 5)
	select {
	case <-fake.appendStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("upload never started")
	}
	pause()
	if sink.ctx.Err() != nil {
		t.Fatal("pause canceled uploads")
	}
	close(held)
	if err := sink.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fake.statements) != 1 {
		t.Fatal("paused rows not published")
	}
}

func TestEncodeChunksSplitsAndRejectsOversizedRow(t *testing.T) {
	batch := testBatch("one", 40)
	defer batch.Release()
	state := &tableState{model: rowmodel.Schema{Fields: []rowmodel.Field{{Name: "id"}, {Name: "value"}}}, definition: tableDefinition{operation: internalColumn{name: "op"}, ordinal: internalColumn{name: "ordinal"}}}
	rows, err := stageRecord(state, batch)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Release()
	payload := ipc.GetSchemaPayload(rows.Schema(), memory.DefaultAllocator)
	defer payload.Release()
	var schema bytes.Buffer
	_, _ = payload.WritePayload(&schema)
	total, chunks := int64(0), 0
	err = encodeChunks(rows, schema.Bytes(), 2000, func(part uploadBatch) error {
		if part.size() > 2000 {
			t.Fatal("oversized chunk")
		}
		total += part.rows
		chunks++
		return nil
	})
	if err != nil || total != 40 || chunks < 2 {
		t.Fatalf("rows=%d chunks=%d error=%v", total, chunks, err)
	}
	if err := encodeChunks(rows, schema.Bytes(), 100, func(uploadBatch) error { return nil }); err == nil {
		t.Fatal("accepted oversized row")
	}
}

func TestAllPoliciesCheckpointOnlyAfterCommit(t *testing.T) {
	for _, policy := range New().Spec().Capabilities.WritePolicies {
		if policy.Durability != filament.DurabilityAfterCommit {
			t.Fatalf("unsafe durability for %s", policy.Mode)
		}
	}
}

func TestIndependentResourcesUploadConcurrently(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "one", "two")
	hold := make(chan struct{})
	fake.holdAppend = hold
	fake.appendStarted = make(chan struct{}, 2)
	// A slow table must not stall the single pipeline writer before it can
	// hand off another resource, even after more than two small batches.
	queued := make(chan struct{})
	go func() {
		for i := 0; i < 10; i++ {
			applyBatch(t, sink, "one", 5)
		}
		applyBatch(t, sink, "two", 5)
		close(queued)
	}()
	select {
	case <-queued:
	case <-time.After(5 * time.Second):
		close(hold)
		t.Fatal("small per-resource queue blocked unrelated resources")
	}
	for i := 0; i < 2; i++ {
		select {
		case <-fake.appendStarted:
		case <-time.After(5 * time.Second):
			close(hold)
			t.Fatal("independent upload serialized behind blocked resource")
		}
	}
	close(hold)
	if err := sink.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestQueuePressureIsBoundedAndCancellable(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "one")
	// An encoded request is larger than half this budget. A second request
	// cannot reserve memory until the first receives an acknowledgement.
	sink.memory = semaphore.NewWeighted(2000)
	fake.holdAppend = make(chan struct{})
	fake.appendStarted = make(chan struct{}, 1)
	applyBatch(t, sink, "one", 5)
	select {
	case <-fake.appendStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("upload never started")
	}
	ctx, cancel := context.WithCancel(context.Background())
	batch := testBatch("one", 5)
	defer batch.Release()
	done := make(chan error, 1)
	go func() {
		_, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: sink.policies["one"]})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("queue accepted more than its memory budget: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancellation succeeded unexpectedly")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queue pressure ignored cancellation")
	}
	_ = sink.Abort(context.Background())
	if !sink.memory.TryAcquire(2000) {
		t.Fatal("queue memory not released")
	}
	sink.memory.Release(2000)
}

func TestCommitCancellationStopsUploads(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "one")
	fake.holdAppend = make(chan struct{})
	fake.appendStarted = make(chan struct{}, 1)
	applyBatch(t, sink, "one", 5)
	select {
	case <-fake.appendStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("upload never started")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sink.Commit(ctx); err == nil {
		t.Fatal("expected canceled commit")
	}
	if len(fake.statements) != 0 {
		t.Fatal("canceled commit published rows")
	}
}

func TestMetadataAddsNullableColumnsWithoutQueryJobs(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "one")
	fake.tables["one"] = json.RawMessage(`{"tableReference":{"projectId":"project","datasetId":"dataset","tableId":"one"},"schema":{"fields":[{"name":"id","type":"INTEGER","mode":"REQUIRED"}]}}`)
	applyBatch(t, sink, "one", 3)
	sink.seal()
	sink.workers.Wait()
	if err := sink.failure(); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	var table struct {
		Schema struct{ Fields []struct{ Name, Mode string } }
	}
	if err := json.Unmarshal(fake.tables["one"], &table); err != nil {
		t.Fatal(err)
	}
	if len(table.Schema.Fields) != 2 || table.Schema.Fields[1].Mode == "REQUIRED" {
		t.Fatalf("schema update=%+v", table)
	}
	if len(fake.statements) != 0 {
		t.Fatal("metadata created a query job")
	}
	var stage struct {
		Expiration int64 `json:"expirationTime,string"`
	}
	if err := json.Unmarshal(fake.tables[sink.tables["one"].stage], &stage); err != nil {
		t.Fatal(err)
	}
	if time.Until(time.UnixMilli(stage.Expiration)) < 6*24*time.Hour {
		t.Fatal("stage missing expiration")
	}
}

func TestIncompatibleDestinationFailsBeforePublication(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "one")
	fake.tables["one"] = json.RawMessage(`{"tableReference":{"projectId":"project","datasetId":"dataset","tableId":"one"},"schema":{"fields":[{"name":"id","type":"STRING"}]}}`)
	applyBatch(t, sink, "one", 3)
	if err := sink.Commit(context.Background()); err == nil || !strings.Contains(err.Error(), "expected INTEGER") {
		t.Fatalf("expected incompatible type failure, got %v", err)
	}
	if len(fake.statements) != 0 || fake.appends != 0 {
		t.Fatal("incompatible schema reached data path")
	}
}

func TestPreparingResourcesDoesNotHoldUploadSlots(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "ready", "starting")
	// Hold the setup permit as if all metadata workers were occupied. The
	// ready table has already created its stream and should still upload.
	sink.setups = semaphore.NewWeighted(1)
	ready := sink.tables["ready"]
	if err := sink.prepareUpload(context.Background(), ready); err != nil {
		t.Fatal(err)
	}
	if err := sink.setups.Acquire(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	defer sink.setups.Release(1)
	sink.uploads = semaphore.NewWeighted(1)
	fake.appendStarted = make(chan struct{}, 1)
	applyBatch(t, sink, "starting", 5)
	applyBatch(t, sink, "ready", 5)
	select {
	case <-fake.appendStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("metadata preparation blocked an established upload")
	}
	// Abort must also release the table waiting for its setup permit.
	if err := sink.Abort(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// Record actual gRPC message sizes at the server, after transport decompression.
// The request still decodes through the same Arrow reader as all other tests.
type appendWireStats struct{ fake *fakeBigQuery }

func (*appendWireStats) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context { return ctx }

func (*appendWireStats) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context {
	return ctx
}
func (*appendWireStats) HandleConn(context.Context, stats.ConnStats) {}
func (s *appendWireStats) HandleRPC(_ context.Context, event stats.RPCStats) {
	payload, ok := event.(*stats.InPayload)
	if !ok {
		return
	}
	if _, ok := payload.Payload.(*storagepb.AppendRowsRequest); !ok {
		return
	}
	s.fake.mu.Lock()
	s.fake.appendLogicalBytes += int64(payload.Length)
	s.fake.appendCompressedBytes += int64(payload.CompressedLength)
	s.fake.mu.Unlock()
}

func TestAppendCompressionReducesWireBytes(t *testing.T) {
	sink, fake := newFixture(t, filament.IngestionFullAppend, "one")
	applyBatch(t, sink, "one", 1000)
	if err := sink.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.appendLogicalBytes == 0 || fake.appendCompressedBytes == 0 {
		t.Fatal("no gRPC payload sizes recorded")
	}
	if fake.appendCompressedBytes >= fake.appendLogicalBytes/2 {
		t.Fatalf("compression ineffective: raw=%d compressed=%d", fake.appendLogicalBytes, fake.appendCompressedBytes)
	}
	t.Logf("synthetic Arrow request: %d bytes before compression, %d transmitted", fake.appendLogicalBytes, fake.appendCompressedBytes)
}

// A new dispatch must publish different rows under the same logical run. A
// redelivery must either reproduce the committed segment or fail closed.
func TestContinuationAndCheckpointRecovery(t *testing.T) {
	for _, changed := range []string{"rows", "checkpoint"} {
		t.Run(changed, func(t *testing.T) {
			sink, fake := newFixture(t, filament.IngestionIncrementalAppend, "one")
			model := sink.tables["one"].model
			policies := sink.policies
			publish := func(execution string, rows int, cursor string) error {
				sink.cancel()
				sink.initialize(context.Background(), filament.RunSpec{Run: "same-run", ExecutionID: execution, WritePolicies: policies})
				if err := sink.EnsureSchema(context.Background(), "one", model); err != nil {
					return err
				}
				batch := testBatch("one", rows)
				defer batch.Release()
				batch.Cursor = &rowmodel.CheckpointData{ResourceName: "one", Cursor: map[string]any{"token": cursor}}
				if _, err := sink.Apply(context.Background(), batch, filament.ApplyOptions{Policy: policies["one"]}); err != nil {
					return err
				}
				sink.seal()
				sink.workers.Wait()
				return sink.publish(context.Background(), sink.tables["one"])
			}
			if err := publish("first", 3, "a"); err != nil {
				t.Fatal(err)
			}
			if err := publish("first", 3, "a"); err != nil {
				t.Fatal(err)
			}
			if len(fake.statements) != 1 {
				t.Fatal("identical replay published twice")
			}
			rows, cursor := 3, "a"
			if changed == "rows" {
				rows = 4
			} else {
				cursor = "b"
			}
			if err := publish("first", rows, cursor); err == nil || !strings.Contains(err.Error(), "segment mismatch") {
				t.Fatalf("changed replay: %v", err)
			}
			if len(fake.statements) != 1 {
				t.Fatal("mismatched replay published")
			}
			if err := publish("resumed", rows, cursor); err != nil {
				t.Fatal(err)
			}
			if len(fake.statements) != 2 {
				t.Fatal("continuation skipped publication")
			}
		})
	}
}
