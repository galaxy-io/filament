import { useEffect, useRef } from "react";

import { useVirtualizer } from "@tanstack/react-virtual";

import {
  PIPELINE_CANVAS_ROUTES_DRAFT_KEY,
  PIPELINE_CANVAS_ROUTES_DRAFT_ROW_HEIGHT,
  PIPELINE_CANVAS_ROUTES_LIST_PADDING_Y,
  PIPELINE_CANVAS_ROUTES_OVERSCAN,
  PIPELINE_CANVAS_ROUTES_ROW_HEIGHT,
} from "@/pages/pipelines/canvas/routes/constants";
import {
  type PipelineCanvasRoutesListItem,
  PipelineCanvasRoutesListItemKind,
} from "@/pages/pipelines/canvas/routes/types";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";

export const usePipelineCanvasRoutesVirtualizer = (
  items: PipelineCanvasRoutesListItem[],
  selectedResourceId: CanvasEdge["id"] | undefined,
) => {
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

  useEffect(() => {
    const index = items.findIndex(
      (item) =>
        item.kind === PipelineCanvasRoutesListItemKind.ROUTE &&
        item.route.edge.id === selectedResourceId,
    );
    if (index !== -1) virtualizer.scrollToIndex(index, { align: "auto" });
  }, [items, selectedResourceId, virtualizer]);

  return { scrollerRef, virtualizer };
};
