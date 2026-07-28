# Plan 05 — Edges: no DOM reads during render

**Goal:** edges never touch the DOM while rendering, and never re-render during pan/zoom.

**Server changes:** none.

**Today:**
- `edges/PipelineCanvasEdge.tsx:41-58` runs `document.querySelector` and three `getBoundingClientRect()` calls **inside render** to figure out where to anchor the edge when a table row is scrolled out of view inside a source node.
- The edge also subscribes to zoom (`useStore(zoomSelector)`), so this DOM-measuring render runs for every edge on **every pan/zoom frame**. `getBoundingClientRect` forces the browser to recalculate layout — that's layout thrash in the hottest path the canvas has.
- It also depends on xyflow's internal DOM classes and magic `data-` attributes, which is fragile.

## The idea

The edge needs two numbers about its source node: where the scrollable table list starts/ends, and where the badge sits. Both are plain pixel offsets **inside the node** — and inside a node, one pixel equals one flow unit, so zoom doesn't matter at all.

So: the node measures itself when things actually change (mount, scroll, resize) and writes the numbers to a small store. The edge just reads numbers. No DOM in render, no zoom math.

## Step 1 — A small measurement store

New file `canvas/nodes/measurements.ts` (plain TS, no React):

```ts
export interface PipelineNodeMeasurements {
  listTop: number;      // px from the top of the node
  listBottom: number;
  badgeCenterY: number;
}

const measurements = new Map<string, PipelineNodeMeasurements>();
const listeners = new Set<() => void>();

export const setPipelineNodeMeasurements = (nodeId: string, next: PipelineNodeMeasurements) => {
  const prev = measurements.get(nodeId);
  if (
    prev &&
    prev.listTop === next.listTop &&
    prev.listBottom === next.listBottom &&
    prev.badgeCenterY === next.badgeCenterY
  ) {
    return;
  }
  measurements.set(nodeId, next);
  for (const listener of listeners) listener();
};

export const removePipelineNodeMeasurements = (nodeId: string) => {
  measurements.delete(nodeId);
  for (const listener of listeners) listener();
};

export const subscribePipelineNodeMeasurements = (listener: () => void) => {
  listeners.add(listener);
  return () => listeners.delete(listener);
};

export const getPipelineNodeMeasurements = (nodeId: string) => measurements.get(nodeId);
```

The "skip if nothing changed" check keeps the returned object stable, which `useSyncExternalStore` needs.

## Step 2 — The source node publishes its own numbers

`PipelineNodeSource` owns the list and badge DOM, so it measures them:

- Put refs on the list and badge elements. Delete the `data-table-list` / `data-resource-badge` attributes — nothing needs them anymore.
- Measure with `offsetTop` / `clientHeight` (layout values — reading them in an event handler is fine, it's render-time reads that thrash):

```ts
const measure = useCallback(() => {
  const list = listRef.current;
  const badge = badgeRef.current;
  if (!list || !badge) return;
  setPipelineNodeMeasurements(id, {
    listTop: list.offsetTop,
    listBottom: list.offsetTop + list.clientHeight,
    badgeCenterY: badge.offsetTop + badge.offsetHeight / 2,
  });
}, [id]);
```

- Call `measure` from: a `useLayoutEffect` after mount and when the table count changes, the list's `onScroll` handler, and a `ResizeObserver` on the node root.
- Cleanup effect calls `removePipelineNodeMeasurements(id)`.
- One thing to check: `offsetTop` is measured from the nearest positioned ancestor. The xyflow node root is absolutely positioned, so it should be the reference — verify no wrapper in between is also positioned. If one is, sum the offsets up to the root.
- Also make sure the node calls `useUpdateNodeInternals` on list scroll, so xyflow moves the row handles. The old rect-based code may have been hiding a missing call.

## Step 3 — Rewrite the edge

The edge becomes a pure function of xyflow's node data plus the store:

```tsx
const PipelineCanvasEdge = ({ id, source, sourceHandleId, sourceX, sourceY, ... }: EdgeProps) => {
  const sourceNode = useInternalNode(source);
  const nodeMeasurements = useSyncExternalStore(
    subscribePipelineNodeMeasurements,
    () => getPipelineNodeMeasurements(source),
  );

  let anchorX = sourceX;
  let anchorY = sourceY;

  const { positionAbsolute } = sourceNode?.internals ?? {};
  const { width, height } = sourceNode?.measured ?? {};
  if (positionAbsolute && width && height) {
    const nodeTop = positionAbsolute.y;
    const nodeRight = positionAbsolute.x + width;
    anchorX = Math.min(sourceX, nodeRight);
    anchorY = clamp(sourceY, nodeTop + PIPELINE_NODE_PADDING, nodeTop + height - PIPELINE_NODE_PADDING);

    const isTableEdge = sourceHandleId && sourceHandleId !== PIPELINE_NODE_SOURCE_HANDLE_ID;
    if (isTableEdge && nodeMeasurements) {
      const listTop = nodeTop + nodeMeasurements.listTop;
      const listBottom = nodeTop + nodeMeasurements.listBottom;
      const isRowVisible = sourceY >= listTop && sourceY <= listBottom;
      if (!isRowVisible) {
        anchorX = nodeRight;
        anchorY = nodeTop + nodeMeasurements.badgeCenterY;
      }
    }
  }

  const [path] = getBezierPath({ sourceX: anchorX, sourceY: anchorY, ... });
  return <BaseEdge id={id} path={path} style={style} markerEnd={markerEnd} />;
};
```

Deleted: the zoom subscription, all `querySelector` calls, the client-rect-to-flow conversion. The row-visibility logic is unchanged — `sourceY` (the handle position) still comes from xyflow, we just compare it against stored numbers instead of fresh rects.

## Verify

1. Visual parity with `main`: connect an edge from a bottom table row, scroll the list — the edge snaps to the badge when the row scrolls out, exactly like before.
2. Performance panel while panning/zooming: no "forced reflow" attributed to the edge, and edge components don't render at all during pure pan/zoom.
3. Add/remove tables so the node resizes — the anchor band stays correct.
4. Delete a source node that has edges — no errors, no leftover store entries.
5. `pnpm typecheck && pnpm lint:check`.

## Done when

- No `querySelector` / `getBoundingClientRect` in any render body under `canvas/`.
- Edges don't re-render on pan/zoom.
- Behavior looks identical to today.
