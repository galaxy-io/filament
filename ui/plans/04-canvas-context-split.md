# Plan 04 — Canvas state: stop re-rendering everything on every change

**Goal:** dragging a node re-renders only that node. Opening the terminal or starting a run re-renders zero nodes.

**Server changes:** none.

**Today:**
- `PipelineCanvasProvider.tsx:50` puts `{ state, dispatch }` in one context. Every dispatch makes a new `state` object, so **every** component using `usePipelineCanvas()` re-renders — including all nodes. Their `memo()` wrappers do nothing, because the context subscription bypasses memo.
- Nodes only need two things: `isReadOnly` and `dispatch`. They currently subscribe to everything.
- `runBindings` and `isActivityOpen` (run/terminal state) live in the same reducer as the graph. Starting a run re-renders the whole canvas; dragging a node re-renders the terminal.

Since we're fine breaking things: delete `usePipelineCanvas()` outright and convert every call site in one pass. No compatibility shim.

## Step 1 — Two contexts instead of one

`dispatch` from `useReducer` never changes identity, so a dispatch-only context never re-renders anyone.

```tsx
const PipelineCanvasStateContext = createContext<PipelineCanvasState>(DEFAULT_STATE);
const PipelineCanvasDispatchContext = createContext<React.Dispatch<PipelineCanvasAction>>(
  () => undefined,
);

export const usePipelineCanvasState = () => useContext(PipelineCanvasStateContext);
export const usePipelineCanvasDispatch = () => useContext(PipelineCanvasDispatchContext);
```

```tsx
<PipelineCanvasDispatchContext.Provider value={dispatch}>
  <PipelineCanvasStateContext.Provider value={state}>
    {children}
  </PipelineCanvasStateContext.Provider>
</PipelineCanvasDispatchContext.Provider>
```

Delete `usePipelineCanvas`, the combined `PipelineCanvasContextShape` type, and the `if (!context) throw` guard (it can never fire — `useContext` always returns the default).

Then grep for `usePipelineCanvas(` and convert every call site to the narrowest hook:
- dispatch-only: `PipelineCanvasControls`, `PipelineCanvasEditWidgetButton`, selector items, node action buttons.
- state (or both): `PipelineCanvas`, terminal, navbar wiring.

## Step 2 — Nodes read `isReadOnly` from node data, not context

This is the decisive fix: nodes must not subscribe to canvas state at all.

- `PipelineCanvas.tsx` already builds `renderedNodes` in a `useMemo`. Put `isReadOnly` into each node's `data` there:

```ts
const renderedNodes = useMemo(
  () => nodes.map((node) => ({ ...node, data: { ...node.data, isReadOnly } })),
  [nodes, isReadOnly],
);
```

- Add `isReadOnly: boolean` to `PipelineConnectionNodeData` in `types.ts`.
- In `PipelineNodeSource` / `PipelineNodeSink` / `PipelineNodePlaceholder`: read `data.isReadOnly`, use `usePipelineCanvasDispatch()` for actions. No state hook.

xyflow compares node objects and only re-renders a node whose own entry changed. Dragging node A no longer touches node B.

## Step 3 — Move run state out of the graph reducer

`runBindings` and `isActivityOpen` update on a different rhythm (run lifecycle) than the graph (editing). Separate them.

- Remove both fields from `PipelineCanvasState`, the reducer, and `actions.ts`.
- New `PipelineCanvasRunProvider` in the canvas dir with its own small state + context pair (`usePipelineCanvasRun`). Mount it inside `PipelineCanvasProvider` in `PipelinePage.tsx`.
- Terminal, activity toggle, and the navbar's run wiring move to `usePipelineCanvasRun()`.

Now a run starting never touches graph state, and a graph edit never re-renders the terminal's log rows.

## Step 4 — Tighten downstream memos

- `PipelineLayoutNavbar.tsx:76` (`hasPipelineGraphChanges`): depend on `state.nodes` and `state.edges`, not the whole state object.
- After converting, profile a drag. If the navbar recomputing per drag frame shows up, compare a cheap digest (node/edge ids + connection ids) instead of the arrays. Only add this if measured — positions are ignored by the comparison anyway (`graph.ts:226-240`).

## If it's still not enough

If profiling still shows wide re-renders after Steps 1–4, replace the state context with a small store (Zustand) and per-component selectors. Don't start there — the steps above fix the systemic problem with no new dependency.

## Verify

1. React DevTools Profiler: drag a node → only that node and the canvas surface render. Sink nodes: zero renders. Terminal: zero renders.
2. Open the terminal / start a run → zero node renders.
3. Preview an old version → nodes switch to read-only (the `data` path works).
4. Full smoke: add node, connect edge, save, run with terminal open.
5. `grep -r "usePipelineCanvas(" src` returns nothing; `pnpm typecheck && pnpm lint:check`.

## Done when

- A node re-renders only when its own data or read-only state changes.
- Run/terminal state lives outside the graph reducer.
- The combined context hook no longer exists.
