# Plan 06 — Smaller wins

Independent items, each ships alone. Server-backed items first — these are the ones where leaning on the server is simpler and safer than doing it in the client.

## Server-backed

### 1. Save conflict detection (server rejects stale saves)

Today the canvas is seeded once at mount. If someone else saves the pipeline while you're editing, your save silently overwrites theirs — `mapCanvasStateToVersionRequest` (`graph.ts:188-197`) sends a possibly-stale `baseVersion` and nobody checks it.

The server is the only place that can decide this reliably:
- In the `CreatePipelineVersion` handler: if `base_version` is not the current latest, return `FAILED_PRECONDITION`.
- Client: on that error, show a conflict dialog — "This pipeline changed since you opened it" — with a reload action. No clever merging; just stop the silent overwrite.
- The proto already carries `base_version`, so this is likely a server behavior change plus one error branch in the client, no proto change.

### 2. Persist node positions in the version proto

Users can drag nodes, but positions are thrown away: saves omit them (`graph.ts:199-208`) and every load re-runs a naive auto-layout (`graph.ts:46-63`). The drag gesture promises persistence that doesn't exist.

Store layout with the data it describes:
- Add to the version node message in the pipelines proto:
  ```proto
  message NodePosition {
    float x = 1;
    float y = 2;
  }
  // on the version node message:
  NodePosition position = N;
  ```
- Server: pass-through storage, no logic.
- Client: `mapCanvasStateToVersionRequest` includes positions; `mapPipelineVersionToCanvasState` uses saved positions and falls back to auto-layout only when unset. Include position in `hasPipelineGraphChanges` (`graph.ts:226-240`) so moving a node marks the canvas dirty.
- `buf generate` + `cd ui && pnpm codegen` after the proto change.

(Alternative: decide dragging shouldn't exist and lock node positions. Either is fine — the current half-state is the only wrong option.)

## Client-only

### 3. Unsaved-changes guard

Navigating away from a dirty canvas silently drops edits; the dirty flag today only disables the version dropdown (`PipelineLayoutNavbar.tsx:176`).
- Use TanStack Router's `useBlocker` where `hasPipelineGraphChanges` lives: block navigation when dirty, confirm with a simple stay/discard dialog.
- Add a `beforeunload` listener (only while dirty) for reloads and tab close.

### 4. Terminal fixes (`canvas/terminal/`, `api/queries/runs.ts`)

- **Keys:** `PipelineCanvasTerminal.tsx:121` uses `key={index}` on a capped buffer. Once full, every new event shifts all indices and React redraws all 2000 rows. Give events a client-side sequence number in the stream reducer and key on that.
- **Ordering:** `runs.ts:117` concatenates per-run buffers, so with two runs the log shows run 1's lines then run 2's. Merge-sort by `atUnixMs` in a `useMemo` before returning.
- **Autoscroll:** `PipelineCanvasTerminal.tsx:97-101` jumps to the bottom on every event, even while you're scrolled up reading. Track "am I near the bottom" in an `onScroll` handler and only autoscroll then.

### 5. Dedupe the repeated page patterns

- **`DeleteEntitySection`** (new, in `src/components/`): wraps `DangerZone` + `Modal` + `DeleteConfirmDialog` + `useDeleteConfirm` behind `{ entityLabel, entityName, dangerTitle, dangerDescription, onDelete, onDeleted }`. Replaces the two identical hand-built copies: `PipelineSettingsForm.tsx:191-209`, `ConnectionDrawer.tsx:120-138`.
- **List-page empty states:** both list pages hand-roll the same three branches — empty, no-search-match, content (`ConnectionsPage.tsx:98-149`, `PipelinesPage.tsx:85-125`). Extend `ListPageShell` with `{ items, renderItems, emptyState, searchEmptyState }` so the branching lives once. Fold in the duplicated "Documentation" button and the primary action that's currently written twice per page.

### 6. One error-toast helper + two one-liners

- Add `showErrorToast(showToast, header, error, fallback)` in a new intent-named util (e.g. `src/utils/toasts.ts`). Replace the ~6 copy-pasted `onError` blocks: `PipelineSettingsForm.tsx:125-131`, `PipelineLayoutNavbar.tsx:92-98,150-156`, `CreateConnectionConfigure.tsx:165-176,213-223`, `PipelinesPage.tsx:70-77`.
- `PipelinesPage.tsx:73` uses raw `error.message` (can be blank) — route through `getErrorMessage`.
- `PipelinesPage.tsx:82` opens the docs link without `"noopener,noreferrer"`. Extract one `openDocs()` used by both pages.

### 7. `version` search param: one owner

It's defined twice with different validation — root schema (`__root.tsx:24-30`) and a hand-written guard in the canvas route (`canvas.tsx:9-12`) — and read from a third place (`PipelinePage.tsx:44`, `from: "__root__"`).
- Define it once, on `/pipelines/$id` (the route that reads it). Delete both other definitions. Read with `useSearch({ from: "/pipelines/$id" })`.
- While there: `@tanstack/zod-adapter` is installed but never used. Use `zodValidator` for the root and `$id` schemas, or uninstall it. Pick one.

### 8. Dependency cleanup

- **`framer-motion`** — zero direct imports (it comes through dls). Remove from `package.json`.
- **`react-github-btn`** — one usage (`MainLayoutNavbar.tsx:85`) that injects a third-party iframe on every page load. Replace with a styled anchor + star icon; drop the dep. (Trade: no live star count.)
- **`pluralize`** — two call sites. Replace with a 3-line helper in `src/utils/format.ts`; drop it and `@types/pluralize`.
- React 19: nothing in app code blocks it; it's gated on the dls peer range. Track separately.

### 9. Tests + CI

- Add `vitest`. First wave is pure functions, no DOM needed: `canvas/reducer.ts`, `create/configure/reducer.ts`, `graph.ts` (state↔request round trip, `hasPipelineGraphChanges`), the refetch-interval helpers in `runs.ts`, `utils/format.ts`.
- `"test": "vitest run"` script; add it and a dedicated `pnpm typecheck` step to `.github/workflows/lint.yaml` (typecheck currently only runs indirectly through the Go embed build).

### 10. Polish

- `useConnectorSpec.ts:6-10`: move the `.find()` into the query's `select` so components only re-render when their connector changes.
- `ConnectionDrawer.tsx:64-66`: kind ternary → a `CONNECTOR_KIND_TO_ROUTE_MAP` in `connectors/constants.ts` (house rule: maps over conditionals).
- `PipelineLayout.tsx:106-119`: sidebar active-item if-chain → route→item map (same rule).
- `PipelineCanvasControls.tsx:64-66`: `setTimeout(0)` before fit-view → an effect keyed on the node change it's waiting for.
- `utils/serialization.ts` monkeypatches `BigInt.prototype.toJSON` but looks like a normal util. Rename to say what it is (e.g. `bigintJsonPatch.ts`) and import it for side effect explicitly in `main.tsx`.
- Query-type nit: the `UseQueryOptions<output, Response>` generics are in the wrong order at `pipelines.ts:38-41`, `connections.ts:39-42`, `runs.ts:76`. Harmless now, breaks typing the moment anyone adds a `select`. Fix in one sweep.
- House-rule bookkeeping: 4 `utils.ts` files exist despite the "no utils.ts" rule. Rename them to intent-named modules, or change the rule to "no grab-bag utils". Either way, stop carrying a rule the code ignores.

## Verify (per item)

`pnpm typecheck && pnpm lint:check`, plus a smoke test of the touched surface (list pages, delete flows, canvas save, terminal during a run). Items 1–2 also need `pnpm codegen` regen and a server-side test for the conflict/position round trip. Item 9 adds `pnpm test` to the loop.
