import { type FC, useMemo } from "react";

import { styled } from "@linaria/react";

import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";

import { usePipelineCanvasIsRunning } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasIsRunning";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import type { PipelineCanvasRoutesDraftState } from "@/pages/pipelines/canvas/routes/hooks/usePipelineCanvasRoutesDraft";
import { usePipelineCanvasRoutesVirtualizer } from "@/pages/pipelines/canvas/routes/hooks/usePipelineCanvasRoutesVirtualizer";
import PipelineCanvasRoutesDraftRow from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesDraftRow";
import PipelineCanvasRoutesEmpty from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesEmpty";
import PipelineCanvasRoutesRow from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesRow";
import {
  type PipelineCanvasRoute,
  type PipelineCanvasRoutesListItem,
  PipelineCanvasRoutesListItemKind,
} from "@/pages/pipelines/canvas/routes/types";
import { getPipelineCanvasRoutesListItems } from "@/pages/pipelines/canvas/routes/utils";

const ListFrame = styled.div`
  flex: 1;
  min-height: 0;
  position: relative;

  display: flex;
  flex-direction: column;
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

const PipelineCanvasRoutesList: FC<PipelineCanvasRoutesListProps> = ({
  routes,
  hasRoutes,
  draftState,
}) => {
  const { selectedNodeId, selectedResourceId, selectResource, clearSelection } =
    usePipelineCanvasSelection();
  const isRunning = usePipelineCanvasIsRunning();

  const items = useMemo(
    () => getPipelineCanvasRoutesListItems(routes, draftState.draft, draftState.resource),
    [routes, draftState.draft, draftState.resource],
  );

  const { scrollerRef, virtualizer } = usePipelineCanvasRoutesVirtualizer(
    items,
    selectedResourceId,
  );

  const selectedGroupKey = useMemo(
    () => routes.find((route) => route.edge.id === selectedResourceId)?.groupKey,
    [routes, selectedResourceId],
  );

  const handleBackgroundClick = () => {
    if (selectedNodeId !== undefined || selectedResourceId !== undefined) clearSelection();
  };
  const onAddSink = draftState.canOpen ? draftState.open : undefined;

  if (items.length === 0) {
    return (
      <ListFrame onClick={handleBackgroundClick}>
        <PipelineCanvasRoutesEmpty hasRoutes={hasRoutes} />
      </ListFrame>
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
    <ListFrame onClick={handleBackgroundClick}>
      <ScrollArea ref={scrollerRef}>
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
      </ScrollArea>
    </ListFrame>
  );
};

export default PipelineCanvasRoutesList;
