# Data layer

Filament talks to the Go server over ConnectRPC. The TypeScript clients are generated from the protos, and every fetch goes through `@connectrpc/connect-query` hooks wrapped once per resource in `src/api/queries/`. Components never call the transport directly.

## From proto to screen

1. **Change or read the proto** under `api/<service>/v1/*.proto`.
2. **Regenerate the clients**: `cd ui && pnpm codegen`. The output in `src/gen/` is committed. A stale `gen/` silently drops request fields, so regenerate before wiring a new field and commit the result.
3. **A new service** needs a Vite proxy entry in `ui/vite.config.ts` (`"/metrics.v1.MetricsService": { target: "http://localhost:8080", changeOrigin: true }`) so the dev server stays same-origin.
4. **Add the query file** `src/api/queries/<resource>.ts` following the contract below.
5. **Use the hooks** from a page or component.

## The plumbing

| File | What it does |
|---|---|
| `api/transport.ts` | `createConnectTransport` with `API_URL` (empty in dev and prod, same origin), binary format in production, a debug logging interceptor in dev. Exports the singleton `transport`. |
| `api/queryClient.ts` | `staleTime` 30s, `refetchOnWindowFocus`, 3 retries except `NotFound`, `PermissionDenied`, `Unauthenticated`, `InvalidArgument`. |
| `api/TransportQueryClientProvider.tsx` | `TransportProvider` + `QueryClientProvider`, mounted once in `main.tsx` inside `GalaxyProvider`. |
| `api/utils.ts` | Pagination (`DEFAULT_PAGE_SIZE` 25, `INITIAL_PAGE_PARAM`, `getNextPageParam`), the shared list search schema and `createListSearchInput`, the option type helpers, `batchIterable` for streams. |
| `api/queries/constants.ts` | `PROBE_QUERY_OPTIONS` (no retry, never stale, no focus refetch, for one-shot checks like auth config and validation) and `ACTIVE_RUN_STATUSES`. |

## The query file contract

One file per resource (`pipelines.ts`, `connections.ts`, `runs.ts`, `connectors.ts`, `schedules.ts`, `notifiers.ts`, `pipeline_versions.ts`, `capabilities.ts`, `metrics.ts`, `auth.ts`). Each exports, per RPC it covers, some of the following. Copy the shape exactly. The generics are verbose on purpose so the hook's `options` type-checks against the response.

```ts
import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  createInfiniteQueryOptions,
  createQueryOptions,
  type UseMutationOptions,
  type UseQueryOptions,
  useMutation,
  useQuery,
  useSuspenseInfiniteQuery,
  useSuspenseQuery,
} from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import type { GetPipelineRequest, GetPipelineResponse, ListPipelinesRequest } from "@/gen/ingestion/v1/pipelines_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

import { createListSearchInput, getNextPageParam, INITIAL_PAGE_PARAM, type InfiniteQueryInput, type ListSearchParams } from "@/api/utils";

export const createListPipelinesInput = (search: ListSearchParams) => ({
  includeLastRun: true,
  includeSchedule: true,
  ...createListSearchInput(search),
});

export const createListPipelinesQueryKey = (input?: ListPipelinesRequest, transport?: Transport) =>
  createConnectQueryKey({
    schema: IngestionService.method.listPipelines,
    input,
    transport,
    cardinality: undefined,
  });

export const createGetPipelineQueryKey = (input?: GetPipelineRequest, transport?: Transport) =>
  createConnectQueryKey({
    schema: IngestionService.method.getPipeline,
    input,
    transport,
    cardinality: "finite",
  });

export const createGetPipelineQueryOptions = ({ input, transport }: { input: GetPipelineRequest; transport: Transport }) =>
  createQueryOptions(IngestionService.method.getPipeline, input, { transport });

export const useGetPipelineQuery = ({
  input,
  options = {},
}: {
  input: GetPipelineRequest;
  options?: UseQueryOptions<typeof IngestionService.method.getPipeline.output, GetPipelineResponse>;
}) =>
  useQuery<typeof IngestionService.method.getPipeline.input, typeof IngestionService.method.getPipeline.output>(
    IngestionService.method.getPipeline,
    input,
    options,
  );

export const useSuspenseGetPipelineQuery = ({ input }: { input: GetPipelineRequest }) =>
  useSuspenseQuery<typeof IngestionService.method.getPipeline.input, typeof IngestionService.method.getPipeline.output>(
    IngestionService.method.getPipeline,
    input,
  );

export const useSuspenseListPipelinesInfiniteQuery = ({
  input,
}: { input?: InfiniteQueryInput<typeof IngestionService.method.listPipelines.input> } = {}) =>
  useSuspenseInfiniteQuery<
    typeof IngestionService.method.listPipelines.input,
    typeof IngestionService.method.listPipelines.output,
    "pagination"
  >(
    IngestionService.method.listPipelines,
    { ...input, pagination: INITIAL_PAGE_PARAM },
    { pageParamKey: "pagination", getNextPageParam },
  );

export const useUpdatePipelineMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.updatePipeline.input,
    typeof IngestionService.method.updatePipeline.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof IngestionService.method.updatePipeline.input,
    typeof IngestionService.method.updatePipeline.output
  >(IngestionService.method.updatePipeline, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({ queryKey: createListPipelinesQueryKey() });
      void queryClient.invalidateQueries({ queryKey: createGetPipelineQueryKey() });
      return options.onSettled?.(...args);
    },
  });
};
```

Naming inside the file.

| Export | Purpose |
|---|---|
| `createXInput(…)` | Builds the request's plain fields from URL state or props. Pages call it so the input is identical everywhere the query is used. |
| `createXQueryKey(input?, transport?)` | For invalidation. Called with no input it matches the whole family. `cardinality: "finite"` for unary RPCs, `undefined` for lists that also have infinite variants. |
| `createXQueryOptions({ input, transport })` | For `beforeLoad` and `useQueries`. Spread `PROBE_QUERY_OPTIONS` over it for one-shot probes. |
| `useXQuery({ input, options })` | Plain query. Resource defaults (`refetchInterval`, `staleTime: Infinity` for immutable catalogs) go before `...options`. |
| `useSuspenseXQuery({ input })` | For pages. Takes no `enabled`. |
| `useXInfiniteQuery` / `useSuspenseXInfiniteQuery` | Server pagination with `pageParamKey: "pagination"`. Pages flatten with `data.pages.flatMap((page) => page.items)` in a `useMemo`. |
| `useXMutation(options = {})` | Wraps `useMutation`, invalidates the affected families in `onSettled`, then chains the caller's `onSettled`. |

Rules.

- **Reference `IngestionService.method.x` directly.** No const aliases for method descriptors.
- **Invalidation lives in the mutation hook, never at the call site.** Invalidate every family the mutation can change (update connection invalidates list, get, discover, columns and validate). Invalidate, do not `setQueryData`.
- **Polling is a function of the data.** `refetchInterval: (query) => getListRunsRefetchInterval(query.state.data?.runs)` returns 3s while a run is active, a wait until the next scheduled run, or `false`. Constants like `LIST_RUNS_REFETCH_INTERVAL` sit at the top of the file.
- **Immutable catalogs** (`ListConnectors`, `GetConnector`) set `staleTime: Infinity` in the hook.
- **Streaming** uses `experimental_streamedQuery` with `batchIterable` (150ms batches), a bounded reducer and `gcTime: 0`, in `useTailRunsStream`. When the server closes the stream the query errors and nothing reconnects. A stale activity feed with `ERR_INCOMPLETE_CHUNKED_ENCODING` in the console means the run ended, not a rendering bug.

## Building requests

Every request is a proto message built with `create` from `@bufbuild/protobuf`. Nested messages can be plain objects inside the init.

```ts
import { create } from "@bufbuild/protobuf";

const input = create(ListRunsRequestSchema, {
  pipelineId: id,
  status: [...ACTIVE_RUN_STATUSES],
  pagination: create(PaginationRequestSchema, { pageSize: 1 }),
});

const request = create(UpdatePipelineRequestSchema, {
  pipelineId: pipeline.id,
  name: state.name.trim(),
  description: state.description.trim(),
  executionMode: pipeline.executionMode,
});
```

- Build an input that depends on props inside `useMemo` when it feeds a hook on every render, or inline for a one-off call.
- Static inputs are module constants (`OBSERVABILITY_CONNECTIONS_INPUT`).
- `include*` flags (`includeVersions`, `includeSchedule`) pick the payload. Different flags are different cache entries.
- **`bigint` everywhere the proto says int64.** Timestamps are unix millis as `bigint`. `src/utils/format.ts` takes `bigint`. Charts want `number`, so bridge with `Number(x)`. `src/utils/serialization.ts` patches `BigInt.prototype.toJSON` so bigints survive the router's search serialization.
- Round-trip proto enums through select ids with `String(value)` and `Number(id) as Role`.
- The proto schema is also a validator. `fromJson(WorkerConfigurationSchema, parsed)` turns user JSON into a message or throws, `toJson` formats it back, `equals(Schema, a, b)` compares.

## Which hook, where

| Surface | Fetch with | Loading shows as |
|---|---|---|
| A route page under `_main` or `pipelines/$id`, and the sections it composes | `useSuspenseXQuery`, `useSuspenseXInfiniteQuery` | The router's `PendingLayout`. Nothing to write. |
| An overlay rendered by `AppLayout` (drawer, modals, settings) | plain `useXQuery` with `enabled` | `PendingLayout` for the whole body, `ErrorLayout` with a Close button on `isError` |
| A dashboard panel | plain `useXQuery` per panel | `isLoading` on the chart or table, a `Skeleton` in a fixed-width `Box` per KPI value |
| A cell or inline value that fetches its own entity (`PipelineName`, `ConnectorTile`) | plain `useXQuery`, `enabled: !initialEntity` | a `Skeleton` sized like the value |
| An expanded table row | plain `useXQuery` | `isError` text, then a `Skeleton`, then content |

- Pages under a route use suspense. The whole island waits once, then everything paints together.
- Dashboards do not use suspense and do not use `keepPreviousData`. Each panel loads on its own and shows a skeleton rather than stale data.
- Nothing rendered above a route (the overlays under `__root`) may suspend. The only boundary above them is app-wide and would blank the whole app.
- **Gate on `isLoading`, never `isPending`.** A disabled query pends forever.
- A definitive 404 (`GetConnection` for a deleted id) passes `retry: false`.
- Loading, empty and error are three different states. Never reuse an empty state as a loading state, never paint an error as an empty state.

## Data ownership

- **Components take ids, not entities, and fetch their own data.** `PipelineName` takes `pipelineId`, a settings section reads `id` from the route and calls `useSuspenseGetPipelineQuery` itself. The cache dedupes, so five sections calling the same query cost one request.
- Entity props survive in three places. A `.map()` row component gets its row. A direct single-hop child gets the object its parent already holds. A genuinely multi-origin presentational component (`PipelineFlow`, `Field`) gets whatever it renders.
- The `{ id, entity? }` shape with `enabled: !initialEntity` lets a row pass what it has and a deep link fetch what it lacks.
- Prefer an entity-scoped Get RPC over deriving one entity from a list on the client. `GetConnector(connector, kind)` was added end to end for this reason.
- Session comes from route context (`useRouteContext({ from: "/_app" })`), not from a query.

## Mutations at the call site

```tsx
const { toast } = useToast();
const { mutate: updatePipeline, isPending: isSaving } = useUpdatePipelineMutation();

const handleSave = () => {
  updatePipeline(request, {
    onSuccess: () => {
      toast({
        header: "Pipeline saved",
        description: `${formatPipelineName(pipeline)} has been saved successfully.`,
        variant: ToastVariant.SUCCESS,
      });
    },
    onError: (error) => {
      toast({
        header: "Save failed",
        description: getErrorMessage(error, "Failed to save pipeline"),
        variant: ToastVariant.ERROR,
      });
    },
  });
};
```

- Destructure and rename, `{ mutate: verbEntity, isPending: isSaving }`. `isPending` drives `isLoading` on the button and `isDismissable` on the dialog.
- Per-call `onSuccess` and `onError` toast. The hook already invalidated.
- `getErrorMessage(error, fallback)` from `src/utils/errors.ts` for the description. Branch on a code with `ConnectError.from(error).code` when it changes the copy (an optimistic-concurrency `Code.Aborted` becomes "Connection changed elsewhere").
- `mutateAsync` for sequences (create pipeline, then its version, then each notifier). When a later step fails, toast "Pipeline created without notifiers" and still navigate.
- Deletes go through `useConfirm` (`src/hooks/useConfirm.ts`), which owns the target, the toasts and the close. See [screens.md](./screens.md).
- Navigate after success only when the thing the user was looking at no longer exists (a delete) or when the result lives elsewhere (a created pipeline opens its canvas).

## Gotchas that have cost real time

- **`queryClient.ensureQueryData` ignores staleness.** It returns cached data whenever it exists. Only `revalidateIfStale: true` changes that. This is why `beforeLoad` only probes auth and nothing else is loaded there.
- **One suspense hook on a key makes `isLoading` dead everywhere else on that key.** `useObservabilitySetup` suspends on `ListConnections({ includeDeleted: true })`, so a plain query on the same input in a runs-table cell is never loading. Check whether a page already suspends on the key before adding skeleton code.
- **DLS `InfiniteTable` prunes `expandedRowIds`** that are not in the loaded rows and calls `onExpandedChange` with the pruned list. When expansion is a search param, a deep link to a row past the first page loses the param. Fetch the row or page until it is loaded before trusting the URL.
- **`refetchOnWindowFocus` is on.** A screen that holds unsaved local state keyed from server data must not reset on refetch. Initialise local state once with `useState(createInitialState)` and reset by `key`, not by syncing in an effect.
