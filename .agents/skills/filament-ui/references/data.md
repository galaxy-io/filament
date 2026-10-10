# Data layer

Filament talks to the Go server over ConnectRPC. The TypeScript clients are generated from the protos, and every fetch goes through `@connectrpc/connect-query` hooks wrapped once per resource in `src/api/queries/`. Components never call the transport directly.

## From proto to screen

1. **Change or read the proto** under `protos/<service>/v1/*.proto`.
2. **Regenerate the clients** with `cd ui && pnpm codegen` (`buf generate`). The output in `src/gen/` is committed. A stale `gen/` silently drops request fields, so regenerate before wiring a new field and commit the result.
3. **A new service** needs its path in `API_SERVICE_PATHS` in `ui/vite.config.ts` so the dev server proxies it same-origin.
4. **Add or extend the query file** `src/api/queries/<resource>.ts` following the contract below.
5. **Use the hooks** from a page or component.

## The plumbing

The transport and the query client belong to the host (`src/host/api/`). Library code reads the transport only through connect-query context with `useTransport()`, so a host can nest its own `TransportProvider`.

| File | What it does |
|---|---|
| `host/api/transport.ts` | `createTransport` over `createConnectTransport`. `VITE_API_URL` or same origin, binary format in production, JSON in dev. `createLoggingInterceptor` is always installed and logs only when `IS_DEBUG`. Exports the singleton `transport`. |
| `host/api/queryClient.ts` | `staleTime` 30s, `refetchOnWindowFocus`, 3 retries. `NON_RETRYABLE_CODES` skips the retry for `NotFound`, `PermissionDenied`, `Unauthenticated`, `InvalidArgument` and `Unimplemented`, so a missing service (no `AuthService` on a bare server) fails at once instead of spinning. |
| `host/api/TransportQueryClientProvider.tsx` | `TransportProvider` + `QueryClientProvider`, mounted once in `host/main.tsx` inside `GalaxyProvider`. |
| `api/utils.ts` | Pagination (`INITIAL_PAGE_PARAM`, `getNextPageParam`), `createListSearchInput` and `createListSortingInput`, the option types `UseInfiniteQueryOptions` and `UseSuspenseQueryOptions`, `InfiniteQueryInput`, and `batchIterable` for streams. |
| `api/queries/constants.ts` | `PROBE_QUERY_OPTIONS`, `ACTIVE_RUNS_REFETCH_INTERVAL`, `ACTIVE_PIPELINE_RUNS_PAGE_SIZE`. |
| `utils/errors.ts` | `getErrorMessage(error, fallback)` returns `rawMessage` for a `ConnectError`, so toasts never show the `[code]` prefix. |

## The query file contract

One file per resource (`pipelines.ts`, `connections.ts`, `runs.ts`, `connectors.ts`, `schedules.ts`, `notifiers.ts`, `pipeline_versions.ts`, `capabilities.ts`, `transforms.ts`, `metrics.ts`, `auth.ts`). Export only what has a consumer. Hooks reference `IngestionService.method.x` directly and let connect-query infer the generics from the method. The `options` parameter is typed with connect-query's option types.

```ts
export const createGetPipelineInput = (id: Pipeline["id"]) =>
  create(GetPipelineRequestSchema, { id, includeVersions: true, includeSchedule: true });

export const createGetPipelineQueryKey = (input?: GetPipelineRequest, transport?: Transport) => {
  return createConnectQueryKey({
    schema: IngestionService.method.getPipeline,
    input,
    transport,
    cardinality: "finite",
  });
};

export const useGetPipelineQuery = ({
  input,
  options = {},
}: {
  input: GetPipelineRequest;
  options?: UseQueryOptions<typeof IngestionService.method.getPipeline.output, GetPipelineResponse>;
}) => {
  return useQuery(IngestionService.method.getPipeline, input, options);
};

export const useSuspenseGetPipelineQuery = ({ input }: { input: GetPipelineRequest }) => {
  return useSuspenseQuery(IngestionService.method.getPipeline, input);
};

export const useUpdatePipelineMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.updatePipeline.input,
    typeof IngestionService.method.updatePipeline.output
  > = {},
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();
  return useMutation(IngestionService.method.updatePipeline, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListPipelinesQueryKey(undefined, transport),
      });
      void queryClient.invalidateQueries({
        queryKey: createGetPipelineQueryKey(undefined, transport),
      });
      return options.onSettled?.(...args);
    },
  });
};
```

Naming inside the file.

| Export | Purpose |
|---|---|
| `createXInput(…)` | Builds the request from an id or URL state so every caller hits the same cache entry. `createGetPipelineInput(id)`, `createGetConnectionInput(id)`, `createListPipelinesInput(search)`, `createListConnectionsInput(kind, search)`. |
| `createXQueryKey(input?, transport?)` | Only when something invalidates it. Called with no input it matches the whole family. |
| `createXQueryOptions` | Only for `useQueries` or the host's `beforeLoad` probes (`createGetAuthConfigQueryOptions`, `createGetSessionQueryOptions`). |
| `useXQuery({ input, options })` | Plain query. Resource defaults go before `...options`. |
| `useSuspenseXQuery({ input })` | For pages. Takes no `enabled`. |
| `useXInfiniteQuery` / `useSuspenseXInfiniteQuery` | Server pagination with `pageParamKey: "pagination"`. Pages flatten with `data.pages.flatMap((page) => page.items)` in a `useMemo`. |
| `useXMutation(options = {})` | Wraps `useMutation`, invalidates in `onSettled`, then chains the caller's `onSettled`. |

Rules.

- **Keys are transport-scoped.** Every mutation hook reads `const transport = useTransport()` and invalidates with `createXQueryKey(undefined, transport)`. Hosts share one QueryClient, and a key without a transport prefix-matches every app's queries for the same service.
- **Cardinality.** Get RPCs use `cardinality: "finite"`. Lists that also have infinite variants (`ListPipelines`, `ListConnections`, `ListRuns`) use `undefined` so one key covers both.
- **Invalidation lives in the mutation hook's `onSettled`, never at a call site.** Invalidate every family the mutation can change. `useUpdateConnectionMutation` invalidates list, get, discover, columns and validate. Invalidate, never `setQueryData`.
- **Polling is a private function of the data.** `getListPipelinesRefetchInterval`, `getListRunsRefetchInterval`, `getGetRunRefetchInterval` and `getActiveRunsRefetchInterval` return `ACTIVE_RUNS_REFETCH_INTERVAL` while a run is active, a wait until the next scheduled run, or `false`. The hook passes `refetchInterval: (query) => getXRefetchInterval(query.state.data?.…)` before `...options`. Metrics poll on a flat `METRICS_REFETCH_INTERVAL`.
- **Catalogs never go stale.** `useListConnectorsQuery`, `useGetConnectorQuery` and `useSuspenseListTransformFunctionsQuery` set `staleTime: Number.POSITIVE_INFINITY` in the hook.
- **Probes** spread `PROBE_QUERY_OPTIONS` (no retry, `networkMode: "always"`, never stale, no focus refetch). Discover, columns and the canvas resource lookups use it, and so does `useGetSessionQuery`. Server validations (`useValidatePipelineQuery`, `useValidateTransformQuery`) set `retry: false` in the hook.
- **Server search.** `createListSearchInput({ q, sortBy, sortOrder })` trims `q` to `MAX_LIST_SEARCH_LENGTH` and sends it as `search`, so a match on an unloaded page is still found. The create-pipeline pane and the canvas selector search the same way.

### Shaped hooks

Some hooks own more than one call. Copy these shapes rather than rebuilding them at a call site.

```ts
const selectConnections = (results: UseQueryResult<GetConnectionResponse>[]) =>
  results.flatMap((result) => result.data?.connection ?? []);

export const useGetConnectionQueries = (ids: Connection["id"][]) => {
  const transport = useTransport();
  return useQueries({
    queries: ids.map((id) =>
      createQueryOptions(IngestionService.method.getConnection, createGetConnectionInput(id), {
        transport,
      }),
    ),
    combine: selectConnections,
  });
};
```

- `useGetConnectionQueries(ids)` resolves several connections by id. The `combine` is module level so its identity is stable. Each id shares the cache entry `useGetConnectionQuery` uses.
- `useListRunsInfiniteQuery` and its suspense twin pass `select: selectUniqueRunPages`, which drops a run already seen on an earlier page when polling shifts the cursor.
- `useListActivePipelineRunsQuery(pipelineId)` and `useSuspenseListActivePipelineRunsQuery(pipelineId)` build their input privately (`status: [...ACTIVE_RUN_STATUSES]`, `ACTIVE_PIPELINE_RUNS_PAGE_SIZE`) and poll on the pipeline schedule's `nextFireAt`.
- `useCanManageTeam({ enabled })` in `api/queries/auth.ts` is a `select` on `ListMembers` that returns `canManage` as `boolean | undefined`. Compare with `=== true` or `=== false`, because `undefined` means not known yet.
- `useRefreshObservabilityQueries()` in `api/queries/metrics.ts` returns a callback that invalidates timeseries, aggregate, runs, connections and pipelines for the current transport.
- `useTailRunsStream(runIds)` uses `experimental_streamedQuery` with `batchIterable` (150ms batches), a reducer capped at 2000 events and `gcTime: 0`. When the server closes the stream the query errors and nothing reconnects.

## Building requests

Every request is a proto message built with `create` from `@bufbuild/protobuf`. Nested messages can be plain objects inside the init.

```ts
const request = create(UpdatePipelineRequestSchema, {
  pipelineId: pipeline.id,
  name: state.name.trim(),
  description: state.description.trim(),
  executionMode: pipeline.executionMode,
});
```

- Static inputs are module constants (`OBSERVABILITY_CONNECTIONS_INPUT`, `OBSERVABILITY_RUNS_SCHEDULED_INPUT`).
- An input that depends on props and feeds a hook every render is built in a `useMemo` or through a `createXInput` helper.
- `include*` flags (`includeVersions`, `includeSchedule`, `includeDeleted`) pick the payload. Different flags are different cache entries, which is why `createGetPipelineInput` fixes them.
- **`bigint` wherever the proto says int64.** Timestamps are unix millis as `bigint`. Charts want `number`, so bridge with `Number(x)`.
- Round-trip proto enums through select ids with `String(value)` and `mapOptionIdToEnum(Enum, id)` from `src/utils/select.ts`.
- The proto schema is also a validator. `fromJson(Schema, parsed)` turns user JSON into a message or throws, and `toJson` formats it back.

## Which hook, where

| Surface | Fetch with | Loading shows as |
|---|---|---|
| A route page and the sections it composes (`PipelinesPage`, `PipelinePage`, `PipelineSettingsPageGeneral`) | `useSuspenseXQuery`, `useSuspenseXInfiniteQuery` | The router's `FilamentPendingComponent`, a DLS `PendingLayout`. Nothing to write. |
| A settings page gated on a permission (`TeamPage`, `ServiceAccountsPage`) | plain `useXQuery` | `isLoading` on the table |
| An overlay mounted by `FilamentLayout` (drawer, modals) | plain `useXQuery` with `enabled` | `PendingLayout` for the body, `ErrorLayout` with a Close button on `isError` |
| A dashboard panel | plain `useXQuery` per panel | `isLoading` on the chart, table or `BigNumber` |
| A cell or inline value that fetches its own entity (`PipelineName`, `PipelineFlowTile`) | plain `useXQuery` | a `Skeleton` in a sized `Box`, or `ConnectorTileShimmer` |
| An expanded table row (`PipelineHistoryRunInfo`) | plain `useXQuery` | `isError` text, then a `Skeleton`, then content |

- Pages suspend once, then everything paints together. There are no route loaders. The host's `beforeLoad` only probes auth config and the session.
- Overlays never suspend. They sit above the route's boundary, so a suspend there would blank the app.
- **Gate on `isLoading`, never `isPending`.** A disabled query pends forever.
- A definitive 404 passes `retry: false` (`ConnectionDrawer`, `EditConnectionModal`).

### Loading, empty and error layouts

Loading, empty and error are three states. Never reuse one as another.

- **Loading.** DLS `PendingLayout` for a whole view (`PendingLayoutSize.SMALL` in a pane), `Skeleton` inside a sized `Box` for a value, a content-shaped shimmer only where the content has a shape (`ConnectorTileShimmer`).
- **Error.** DLS `ErrorLayout` with an `icon`, a `header`, a `description` and the debug text only in development.

```tsx
<ErrorLayout
  size={ErrorLayoutSize.SMALL}
  header={message}
  detail={IS_DEBUG ? error.message : undefined}
  actions={actions}
/>
```

- **Empty.** DLS `EmptyLayout`, with `EmptyLayoutSize.SMALL` inside a panel or table. A list has two empty states. The first-run state explains the feature and offers the create action. The filtered state says nothing matched.

```tsx
if (!kindConnections.length && !q) {
  return <EmptyLayout graphic={<EmptyGraphic />} header={`No ${kindPlural} found`} … />;
}

if (!kindConnections.length) {
  return <EmptyLayout icon={MagnifyingGlassIcon} header={`No ${kindPlural} match your search`} />;
}
```

## Data ownership

- **Entities self-fetch by id.** `PipelineName` takes `pipelineId`, `PipelineFlowTile` takes `connectionId`, and each calls its own Get query. The cache dedupes, so twenty rows naming the same pipeline cost one request.
- **Never resolve an entity from the first page of a list.** A list holds 25 rows. A lookup through it fails silently for row 26. Use the Get query, or `useGetConnectionQueries(ids)` for several.
- Entity props survive in three places. A `.map()` row gets its row. A direct child gets the object its parent already holds (`ConnectionDrawerHeader`). A presentational component with several origins (`PipelineFlow`, `Field`) gets what it renders.
- The `{ pipelineId, pipeline? }` shape with `enabled: !initialPipeline` lets a row pass what it has and a deep link fetch what it lacks.
- The host reads the session from the `/_app` route context. Library pages cannot import host routes, so they call `useGetSessionQuery()` (`TeamPage`).

## Mutations at the call site

```tsx
const { toast } = useToast();
const { mutate: updatePipeline, isPending: isSaving } = useUpdatePipelineMutation();

const handleSave = () => {
  updatePipeline(request, {
    onSuccess: () => {
      toast({
        header: "Pipeline saved",
        description: "Your pipeline has been saved successfully.",
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

- Destructure and rename, `const { mutate: verbEntity, isPending } = useXMutation()`. `isPending` drives `isLoading` on the button.
- Toast per call in `onSuccess` and `onError`. The hook already invalidated.
- Branch on a code with `ConnectError.from(error).code` only when it changes the copy. In `EditConnectionModal` a `Code.Aborted` becomes "Connection changed elsewhere".
- `mutateAsync` only for sequences. `CreatePipelineModalFooter` creates the pipeline, then its version, then each notifier, toasts "Pipeline created without all notifiers" when a later step fails, and still navigates.
- Deletes go through `useConfirm` (`src/hooks/useConfirm.ts`), which owns the target, the toasts and the close. See [screens.md](./screens.md).
- Navigate after success only when the thing on screen is gone (a delete) or the result lives elsewhere (a created pipeline opens its canvas).

## Gotchas that have cost real time

- **`queryClient.ensureQueryData` ignores staleness.** It returns cached data whenever it exists. This is why `beforeLoad` only probes auth and nothing else loads there.
- **One suspense hook on a key makes `isLoading` dead everywhere else on that key.** `useObservabilitySetup` suspends on `OBSERVABILITY_CONNECTIONS_INPUT` and `OBSERVABILITY_PIPELINES_INPUT`, so `ObservabilityTimeseriesChart`'s plain `useListPipelinesQuery` on the same input is never loading. Check whether a page already suspends on a key before writing skeleton code for it.
- **`refetchOnWindowFocus` is on.** A screen that holds unsaved local state seeded from server data must not reset on refetch. Seed it once with `useState(createInitialState)` and reset by `key`, never by syncing in an effect. See [forms.md](./forms.md).
- **An unfocused tab pauses retries.** TanStack waits for focus before retrying, so a retryable failure can look like an empty table. Non-retryable codes fail at once.
