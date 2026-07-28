# Plan 03 — List pages: constant request count, precise cache invalidation

**Goal:** rendering a list of N pipelines costs 2 requests, no matter what N is. Saving one pipeline only refreshes that pipeline's cache.

**Today:**
- `PipelineCard.tsx:77-80` fetches `getPipelineVersion` once **per card**, and `usePipelineConnectionMap.ts:17-25` fetches it once **per pipeline**. 50 pipelines = 50 requests to draw a list, and they all fire again on window focus once stale.
- The cards only need a tiny slice of the version: which connections it touches and whether it has edges.
- `pipeline_versions.ts:100-113`: saving a version invalidates `getPipelineVersion` / `listPipelineVersions` / `getPipeline` with **no id** — that wipes the cache for every pipeline, so the next list render refires all N requests.
- `runs.ts:136-141`: running a pipeline only invalidates `listRuns`. The card's "Last run" and "Volume" values come from `listPipelines`, which stays stale.
- The same `listConnections` data is cached under 4 different keys because call sites pass 4 different input shapes.

## Server change (this is the fix — don't work around it client-side)

The list endpoint should carry what the list page renders. Add a per-pipeline summary of the current version to `ListPipelines`:

```proto
message ListPipelinesResponse {
  repeated PipelineSummary pipelines = 1;
}

message PipelineSummary {
  Pipeline pipeline = 1;
  repeated NodeSummary nodes = 2;  // from the current version
  bool has_edges = 3;
}

message NodeSummary {
  string connection_id = 1;
  ConnectorKind kind = 2;
}
```

Why the server is the right place:
- One list query with a join beats N follow-up RPCs. The database can do this in one pass; the client can't.
- The client stops caching N version entries just to throw most of each away.
- This is a breaking proto change to `ListPipelinesResponse` — that's fine, change the shape properly rather than adding a parallel field.

Steps: update the proto in `api/`, implement the join in the server list handler, `buf generate`, then `cd ui && pnpm codegen`.

## Client changes

### 1. Consume the summary

- `PipelineCard.tsx`: delete its `useGetPipelineVersionQuery` and `useListConnectionsQuery`. The card receives `nodes` / `hasEdges` (and resolved connection names) as props from `PipelinesPage`, which already holds the list response and fetches `listConnections` once.
- `usePipelineConnectionMap.ts`: delete the `useQueries` fan-out. Build the pipeline→connection-ids map straight from the list response.

Result: `/pipelines` and `/sources`+`/sinks` cost exactly 2 RPCs: `listPipelines` + `listConnections`.

### 2. Scope save invalidation to the saved pipeline

With Plan 02's merged `GetPipelineResponse`, there are only two keys to touch. In `pipeline_versions.ts`, `onSettled` gets the pipeline id from the mutation input:

```ts
onSettled: (data, error, variables, context) => {
  void queryClient.invalidateQueries({ queryKey: createListPipelinesQueryKey() });
  void queryClient.invalidateQueries({
    queryKey: createGetPipelineQueryKey(
      create(GetPipelineRequestSchema, { id: variables?.pipelineId ?? "" }),
    ),
  });
  return options.onSettled?.(data, error, variables, context);
},
```

Saving pipeline A no longer refetches anything for pipeline B.

### 3. Refresh run metrics after a run

In `runs.ts` `useRunPipelineMutation`, add next to the `listRuns` invalidation:

```ts
void queryClient.invalidateQueries({ queryKey: createListPipelinesQueryKey() });
void queryClient.invalidateQueries({
  queryKey: createGetPipelineQueryKey(
    create(GetPipelineRequestSchema, { id: variables?.pipelineId ?? "" }),
  ),
});
```

"Last run" and "Volume" on the card now update after a run without a manual refresh. (Mid-run updates still ride the 30s staleness window — fine for rollup numbers.)

### 4. One cache entry for connections

Fetch the full connection list once and filter in the client. Server-side `kind` filtering saves nothing here (the list is small) and costs us split caches.

- `api/queries/connections.ts`: hooks always build the same input internally (the empty message — never `undefined`). Callers that want a kind pass it as a plain arg and the hook applies `select`:

```ts
export const useSuspenseListConnectionsQuery = (kind?: ConnectorKind) =>
  useSuspenseQuery(IngestionService.method.listConnections, LIST_CONNECTIONS_INPUT, {
    select: kind === undefined
      ? undefined
      : (data) => ({ ...data, connections: data.connections.filter((c) => c.kind === kind) }),
  });
```

- Update the 4 call sites (`PipelinePage`, `PipelineCard` — deleted anyway in step 1, `ConnectionsPage`, `PipelineCanvasConnectionSelector`) to the new signature.
- Optional server cleanup: drop the now-unused `kind` filter from `ListConnectionsRequest`. Fewer code paths to maintain.

Result: one cache entry; creating or deleting a connection updates every view at once.

## Verify

1. Network tab on `/pipelines` with many pipelines: exactly 2 RPCs.
2. Save a version on pipeline A: no `GetPipeline` refetch for pipeline B (React Query devtools or Network tab).
3. Run a pipeline: the card's "Last run" updates without a refresh.
4. Create a connection: `/sources`, `/sinks`, and the canvas selector all show it.
5. `pnpm codegen` diff reviewed; `pnpm typecheck`.

## Done when

- List request count doesn't grow with pipeline count.
- Save invalidation touches one pipeline.
- Connections live under one cache key.
