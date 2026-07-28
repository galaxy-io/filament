# Plan 02 — Pipeline detail page: one round trip, preload on hover

**Goal:** opening `/pipelines/$id` costs one or two parallel requests instead of four in a row, and the request starts on hover, before the click.

**Today:**
- `src/pages/pipelines/PipelinePage.tsx:27-38` makes 4 requests, one after another: `getPipeline` → `listPipelineVersions` → `listConnections` → `getPipelineVersion`. Each suspense hook blocks the next. Page load time = the sum of all four.
- The 4th query also has a hack: `retry: false` plus a manual `isPending` check that shows a second loading screen (`:63-65`), because a pipeline might not have a version yet.
- `src/router.tsx` has no `defaultPreload`, so nothing prefetches on hover.

## Server change (do this — it's the better fix)

These four pieces of data always load together. The server should return them together.

Extend `GetPipelineResponse` in the pipelines proto:

```proto
message GetPipelineResponse {
  Pipeline pipeline = 1;
  PipelineVersion current_version = 2;   // unset if the pipeline has no version yet
  repeated PipelineVersion versions = 3; // newest first, for the version selector
}
```

Why the server is the right place:
- The server already has all three in one database neighborhood; joining them there is one query. The client doing it is three requests plus cache juggling.
- "No version yet" becomes a normal unset field instead of a client-side `retry: false` + tolerated error.
- Cache invalidation gets simple: one key per pipeline (`getPipeline`) instead of three keys that must be invalidated together (see Plan 03).

Steps: update the proto in `api/`, implement the join in the server handler, run `buf generate`, then `cd ui && pnpm codegen` (stale generated files silently drop new fields — always regen).

## Client changes

### 1. Export the query client

In `src/api/TransportQueryClientProvider.tsx`:

```ts
export const queryClient = new QueryClient({ ... });
```

Both `queryClient` and `transport` (`api/transport.ts:57`) are plain module values, so route loaders can import them directly. No router-context plumbing needed.

### 2. Add option factories

In `api/queries/pipelines.ts` and `connections.ts`, mirror the existing pattern from `pipeline_versions.ts:61-68`:

```ts
export const createGetPipelineQueryOptions = (input: GetPipelineRequest, transport: Transport) =>
  createQueryOptions(IngestionService.method.getPipeline, input, { transport });
```

(Same for `listConnections`.)

### 3. Loader on the route

`src/routes/pipelines/$id.tsx`:

```tsx
export const Route = createFileRoute("/pipelines/$id")({
  loader: ({ params }) =>
    Promise.all([
      queryClient.ensureQueryData(
        createGetPipelineQueryOptions(create(GetPipelineRequestSchema, { id: params.id }), transport),
      ),
      queryClient.ensureQueryData(createListConnectionsQueryOptions(undefined, transport)),
    ]),
  component: PipelinePage,
});
```

Two requests, fired at the same time. The page's suspense hooks then read from warm cache.

### 4. Simplify `PipelinePage`

- Delete `useSuspenseListPipelineVersionsQuery` and `useGetPipelineVersionQuery` — `versions` and `version` now come from the one `useSuspenseGetPipelineQuery` response.
- Delete the `isVersionPending` check and the second `PendingLayout` (`:35-38`, `:63-65`).
- `version` unset means "empty canvas" — same behavior as today, no special casing.

### 5. Preload on hover

In `src/router.tsx`:

```ts
export const router = createRouter({
  routeTree,
  defaultPreload: "intent",
  defaultPreloadStaleTime: 0,
  ...
});
```

`defaultPreloadStaleTime: 0` means the router always runs the loader and lets React Query decide freshness (its 30s `staleTime` dedupes). Hovering a link runs the loader before the click.

One catch: `PipelineCard.tsx:92-97` navigates with `useNavigate` on a div click. Hover preload only works on `<Link>`. Change the card wrapper to a styled `Link` — this is the most-clicked navigation in the app, so it's the one that matters.

### 6. Same pattern for list pages (small, do it while here)

Add one-line loaders to `_main/pipelines`, `_main/sources`, `_main/sinks` so hover preload works on the navbar too.

## Verify

1. Cold load of `/pipelines/$id` in the Network tab: two RPCs, fired together — not four stacked.
2. No second loading flash between the shell and the canvas.
3. Hover a pipeline card for a moment, then click: page appears with no spinner.
4. A pipeline with no versions still opens to an empty canvas.
5. `pnpm typecheck && pnpm lint:check`.

## Done when

- Detail page load time ≈ the slowest single request, not the sum.
- `listPipelineVersions` and the standalone `getPipelineVersion` client hooks are deleted.
- Hover prefetch works on cards and navbar links.
