import { useEffect, useMemo, useRef } from "react";

import { styled } from "@linaria/react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { usePipelineCanvasIsRunning } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasIsRunning";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import {
  PIPELINE_CANVAS_ROUTES_DRAFT_KEY,
  PIPELINE_CANVAS_ROUTES_DRAFT_ROW_HEIGHT,
  PIPELINE_CANVAS_ROUTES_LIST_PADDING_Y,
  PIPELINE_CANVAS_ROUTES_OVERSCAN,
  PIPELINE_CANVAS_ROUTES_ROW_HEIGHT,
} from "@/pages/pipelines/canvas/routes/constants";
import type { PipelineCanvasRoutesDraftState } from "@/pages/pipelines/canvas/routes/hooks/usePipelineCanvasRoutesDraft";
import PipelineCanvasRoutesDraftRow from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesDraftRow";
import PipelineCanvasRoutesEmpty from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesEmpty";
import PipelineCanvasRoutesRow from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesRow";
import {
  type PipelineCanvasRoute,
  type PipelineCanvasRoutesListItem,
  PipelineCanvasRoutesListItemKind,
} from "@/pages/pipelines/canvas/routes/types";
import { getPipelineCanvasRoutesListItems } from "@/pages/pipelines/canvas/routes/utils";

const ListScroller = styled.div`
  flex: 1;
  min-height: 0;
  position: relative;
  overflow: auto;
`;

const ListInner = styled.div`
  position: relative;
  width: 100%;
`;

const ListRowSlot = styled.div`
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
`;

interface PipelineCanvasRoutesListProps {
  routes: PipelineCanvasRoute[];
  hasRoutes: boolean;
  draftState: PipelineCanvasRoutesDraftState;
}

const PipelineCanvasRoutesList = ({
  routes,
  hasRoutes,
  draftState,
}: PipelineCanvasRoutesListProps) => {
  const { selectedNodeId, selectedResourceId, selectResource, clearSelection } =
    usePipelineCanvasSelection();
  const isRunning = usePipelineCanvasIsRunning();

  const items = useMemo(
    () => getPipelineCanvasRoutesListItems(routes, draftState.draft, draftState.resource),
    [routes, draftState.draft, draftState.resource],
  );

  const scrollerRef = useRef<HTMLDivElement>(null);
  const virtualizer = useVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: items.length,
    getScrollElement: () => scrollerRef.current,
    estimateSize: (index) =>
      items[index].kind === PipelineCanvasRoutesListItemKind.DRAFT
        ? PIPELINE_CANVAS_ROUTES_DRAFT_ROW_HEIGHT
        : PIPELINE_CANVAS_ROUTES_ROW_HEIGHT,
    getItemKey: (index) => {
      const item = items[index];
      return item.kind === PipelineCanvasRoutesListItemKind.ROUTE
        ? item.route.edge.id
        : PIPELINE_CANVAS_ROUTES_DRAFT_KEY;
    },
    overscan: PIPELINE_CANVAS_ROUTES_OVERSCAN,
    paddingStart: PIPELINE_CANVAS_ROUTES_LIST_PADDING_Y,
    paddingEnd: PIPELINE_CANVAS_ROUTES_LIST_PADDING_Y,
  });

  const selectedGroupKey = useMemo(
    () => routes.find((route) => route.edge.id === selectedResourceId)?.groupKey,
    [routes, selectedResourceId],
  );

  useEffect(() => {
    const index = items.findIndex(
      (item) =>
        item.kind === PipelineCanvasRoutesListItemKind.ROUTE &&
        item.route.edge.id === selectedResourceId,
    );
    if (index !== -1) virtualizer.scrollToIndex(index, { align: "auto" });
  }, [items, selectedResourceId, virtualizer]);

  const handleBackgroundClick = () => {
    if (selectedNodeId !== undefined || selectedResourceId !== undefined) clearSelection();
  };
  const onAddSink = draftState.canOpen ? draftState.open : undefined;

  if (items.length === 0) {
    return (
      <ListScroller onClick={handleBackgroundClick}>
        <PipelineCanvasRoutesEmpty hasRoutes={hasRoutes} />
      </ListScroller>
    );
  }

  const renderItem = (item: PipelineCanvasRoutesListItem) => {
    if (item.kind === PipelineCanvasRoutesListItemKind.DRAFT) {
      return <PipelineCanvasRoutesDraftRow draftState={draftState} />;
    }
    const { route } = item;
    return (
      <PipelineCanvasRoutesRow
        route={route}
        isSelected={route.edge.id === selectedResourceId}
        isGroupSelected={route.groupKey === selectedGroupKey}
        isRunning={isRunning}
        onSelect={selectResource}
        onAddSink={
          route.isNamedResource && route.groupSize < draftState.sinkCount ? onAddSink : undefined
        }
      />
    );
  };

  return (
    <ListScroller ref={scrollerRef} onClick={handleBackgroundClick}>
      <ListInner style={{ height: virtualizer.getTotalSize() }}>
        {virtualizer.getVirtualItems().map((item) => (
          <ListRowSlot
            key={item.key}
            style={{ height: item.size, transform: `translateY(${item.start}px)` }}
          >
            {renderItem(items[item.index])}
          </ListRowSlot>
        ))}
      </ListInner>
    </ListScroller>
  );
};

export default PipelineCanvasRoutesList;
