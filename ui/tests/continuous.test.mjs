import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

// Use the application's existing TS/alias loader, with no new test dependencies.
const vite = await createServer({
  configFile: false,
  root: fileURLToPath(new URL("../", import.meta.url)),
  resolve: { alias: { "@": fileURLToPath(new URL("../src", import.meta.url)) } },
  server: { middlewareMode: true, ws: false, watch: null },
  optimizeDeps: { noDiscovery: true, include: [] },
  ssr: { noExternal: ["@galaxy-io/dls"] },
});
after(() => vite.close());
const load = (path) => vite.ssrLoadModule(`/src/${path}`);
const { ExecutionMode, ReadMode, ReplicationMode, WriteMode, ConnectorKind } = await load("gen/ingestion/v1/common_pb.ts");
const { ExecutionDesiredState, ExecutionObservedState, RunSignal, RunStatus } = await load("gen/ingestion/v1/runs_pb.ts");
const { parseStreamResources, isContinuousRunActive, runPauseSignal, runStopSignal } = await load("pages/pipelines/streaming.ts");
const { mapCreatePipelineStateToRequest, mapCreatePipelineStateToVersionRequest } = await load("pages/pipelines/components/create/serialize.ts");
const { mapCanvasStateToVersionRequest, mapPipelineVersionToCanvasState } = await load("pages/pipelines/canvas/graph/serialize.ts");

const { hasPipelineGraphChanges } = await load("pages/pipelines/canvas/graph/diff.ts");
const { commonExecutionModes } = await load("pages/pipelines/streaming.ts");
const { buildResourceRowsBySink } = await load("pages/pipelines/components/create/rows.ts");

const source = { id: "src", kind: ConnectorKind.SOURCE };
const sink = { connection: { id: "sink", kind: ConnectorKind.SINK }, writeMode: WriteMode.APPEND };
const rows = ["orders.>", "products.>"].map((name) => ({ name, isSelected: true, isSelectable: true, readMode: ReadMode.FULL, cursorField: "old_cursor" }));
const versionInput = { sourceConnection: source, sinks: [sink], rowsBySink: { sink: rows }, nodeConfigs: { src: { source_identity: "test" } }, replication: ReplicationMode.STANDARD, pipelineId: "pipeline" };

test("continuous resources remain explicit and remove bounded read settings", () => {
  const result = mapCreatePipelineStateToVersionRequest({ ...versionInput, executionMode: ExecutionMode.CONTINUOUS });
  assert.deepEqual(result.graph.edges.map((edge) => edge.resource), ["orders.>", "products.>"]);
  for (const edge of result.graph.edges) {
    assert.equal(edge.readMode, ReadMode.UNSPECIFIED);
    assert.equal(edge.writeMode, WriteMode.APPEND);
    assert.deepEqual(edge.cursors, []);
  }
  assert.deepEqual(result.graph.nodes[0].config, { source_identity: "test" });
  const roundTrip = mapCanvasStateToVersionRequest(mapPipelineVersionToCanvasState({ graph: result.graph }), "pipeline", { graph: result.graph }, ExecutionMode.CONTINUOUS);
  assert.deepEqual(roundTrip.graph.edges.map((edge) => edge.resource), ["orders.>", "products.>"]);
  assert.ok(roundTrip.graph.edges.every((edge) => edge.readMode === ReadMode.UNSPECIFIED && edge.cursors.length === 0));
});

test("bounded all-resources serialization is preserved", () => {
  const result = mapCreatePipelineStateToVersionRequest({ ...versionInput, executionMode: ExecutionMode.BOUNDED });
  assert.equal(result.graph.edges.length, 1);
  assert.equal(result.graph.edges[0].resource, "");
  assert.equal(result.graph.edges[0].readMode, ReadMode.FULL);
});

test("continuous creation never submits a cron schedule", () => {
  const state = { executionMode: ExecutionMode.CONTINUOUS, description: "", workerConfiguration: "{}", schedule: { isEnabled: true } };
  const request = mapCreatePipelineStateToRequest(state, "Events");
  assert.equal(request.executionMode, ExecutionMode.CONTINUOUS);
  assert.equal(request.schedule, undefined);
});

test("resource input accepts domains, topics and endpoints without discovery", () => {
  assert.deepEqual(parseStreamResources(" orders.>\r\nproducts.>\norders.>\n\nhttps://hooks.example/events?a=1,2\n"), ["orders.>", "products.>", "https://hooks.example/events?a=1,2"]);
});

test("signals use desired state and stop semantics; draining remains active", () => {
  const run = { executionMode: ExecutionMode.CONTINUOUS, status: RunStatus.RUNNING, executionStatus: { desiredState: ExecutionDesiredState.PAUSED, observedState: ExecutionObservedState.DRAINING } };
  assert.equal(runPauseSignal(run), RunSignal.RESUME);
  assert.equal(runStopSignal(run), RunSignal.STOP);
  assert.equal(isContinuousRunActive(run), true);
  run.executionStatus.desiredState = ExecutionDesiredState.STOPPED;
  assert.equal(isContinuousRunActive(run), true);
  run.executionStatus.observedState = ExecutionObservedState.BLOCKED;
  assert.equal(isContinuousRunActive(run), true);
  run.executionStatus.observedState = ExecutionObservedState.STOPPED;
  assert.equal(isContinuousRunActive(run), false);
  assert.equal(runStopSignal({ executionMode: ExecutionMode.BOUNDED }), RunSignal.CANCEL);
  assert.equal(runPauseSignal({ executionMode: ExecutionMode.BOUNDED, status: RunStatus.PAUSED }), RunSignal.RESUME);
});


test("execution choices follow every route's capabilities", () => {
  assert.equal(commonExecutionModes([]), undefined);
  assert.deepEqual(commonExecutionModes([{ supportedExecutionModes: [ExecutionMode.CONTINUOUS] }]), [ExecutionMode.CONTINUOUS]);
  assert.deepEqual(commonExecutionModes([
    { supportedExecutionModes: [ExecutionMode.BOUNDED, ExecutionMode.CONTINUOUS] },
    { supportedExecutionModes: [ExecutionMode.CONTINUOUS] },
  ]), [ExecutionMode.CONTINUOUS]);
  assert.deepEqual(commonExecutionModes([
    { supportedExecutionModes: [ExecutionMode.BOUNDED] },
    { supportedExecutionModes: [ExecutionMode.CONTINUOUS] },
  ]), []);
  assert.deepEqual(commonExecutionModes([{ supportedExecutionModes: [] }]), []);
});

test("continuous resource rows do not require a bounded read mode", () => {
  const state = {
    executionMode: ExecutionMode.CONTINUOUS,
    sinkConnections: [{ id: "sink" }],
    sinkWriteModes: { sink: WriteMode.APPEND },
    resourceSelection: {}, resourceReadModes: {}, resourceCursors: {},
  };
  const input = {
    state, resources: [{ name: "orders.>", isSelectable: true, metadata: {}, primaryKey: [] }],
    columns: undefined, supportedReadModesBySink: {}, isCdc: false,
  };
  assert.equal(buildResourceRowsBySink(input).sink[0].status, undefined);
  state.resourceSelection = { sink: { "orders.>": false } };
  assert.equal(buildResourceRowsBySink(input).sink[0].isSelected, false);
  state.sinkConnections.push({ id: "other" });
  assert.equal(buildResourceRowsBySink(input).sink[0].isSelected, false);
  state.resourceSelection = {};
  state.executionMode = ExecutionMode.BOUNDED;
  assert.match(buildResourceRowsBySink(input).sink[0].status.message, /selected read mode/);
});


test("destination labels survive creation and canvas round trips without changing source identity", () => {
  const rows = versionInput.rowsBySink.sink.map((row, index) => ({ ...row, subject: row.name, destinationResource: index === 0 ? "orders" : "products" }));
  const result = mapCreatePipelineStateToVersionRequest({ ...versionInput, rowsBySink: { sink: rows }, executionMode: ExecutionMode.CONTINUOUS });
  assert.deepEqual(result.graph.edges.map((edge) => [edge.resource, edge.destinationResource]), [["orders.>", "orders"], ["products.>", "products"]]);
  const canvas = mapPipelineVersionToCanvasState({ graph: result.graph });
  assert.equal(hasPipelineGraphChanges(canvas, { graph: result.graph }), false);
  canvas.edges[0].data.destinationResource = "order_events";
  assert.equal(hasPipelineGraphChanges(canvas, { graph: result.graph }), true);
  const roundTrip = mapCanvasStateToVersionRequest(canvas, "pipeline", { graph: result.graph }, ExecutionMode.CONTINUOUS);
  assert.equal(roundTrip.graph.edges[0].resource, "orders.>");
  assert.equal(roundTrip.graph.edges[0].destinationResource, "order_events");
});


test("manual stream rows retain selection while labels and subjects are edited", async () => {
  const { default: reducer } = await load("pages/pipelines/components/create/reducer.ts");
  const { CreatePipelineModalActionType: Action } = await load("pages/pipelines/components/create/actions.ts");
  let state = { manualStreamResources: [], resourceSelection: { sink: { discovered: false } }, streamResourceEdits: {} };
  state = reducer(state, { type: Action.ADD_STREAM_RESOURCE, payload: { id: "row-1", sinkId: "sink" } });
  state = reducer(state, { type: Action.SET_STREAM_RESOURCE, payload: { id: "row-1", label: "orders", subject: "orders.>" } });
  assert.deepEqual(state.manualStreamResources, ["row-1"]);
  assert.deepEqual(state.resourceSelection.sink, { discovered: false, "row-1": true });
  assert.deepEqual(state.streamResourceEdits["row-1"], { label: "orders", subject: "orders.>" });
});
