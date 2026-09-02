import { useMemo } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";

import { PipelineCanvasPanelTab } from "@/pages/pipelines/canvas/panel/types";
import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";

const PIPELINE_CANVAS_ROUTE = "/_app/pipelines/$id/canvas";

interface PipelineCanvasSearch {
  node?: CanvasNode["id"];
  resource?: CanvasEdge["id"];
  showPanel?: boolean;
  tab?: PipelineCanvasPanelTab;
}

export const usePipelineCanvasSelection = () => {
  const navigate = useNavigate();
  const { node, resource, showPanel, tab } = useSearch({ from: PIPELINE_CANVAS_ROUTE });

  return useMemo(() => {
    const setSearch = (patch: PipelineCanvasSearch) =>
      void navigate({
        to: ".",
        search: (prev) => ({ ...prev, ...patch }),
        replace: true,
      });

    return {
      selectedNodeId: node,
      selectedResourceId: resource,
      showPanel: showPanel ?? false,
      activeTab: tab ?? PipelineCanvasPanelTab.OVERVIEW,
      selectNode: (nodeId: CanvasNode["id"]) =>
        setSearch({ node: nodeId, resource: undefined, showPanel: true, tab: undefined }),
      selectResource: (edgeId: CanvasEdge["id"]) =>
        setSearch({ node: undefined, resource: edgeId, showPanel: true, tab: undefined }),
      clearSelection: () => setSearch({ node: undefined, resource: undefined }),
      setShowPanel: (nextShowPanel: boolean) =>
        setSearch({ showPanel: nextShowPanel || undefined }),
      setActiveTab: (nextTab: PipelineCanvasPanelTab) =>
        setSearch({
          tab: nextTab === PipelineCanvasPanelTab.OVERVIEW ? undefined : nextTab,
          node: undefined,
          resource: undefined,
        }),
    };
  }, [navigate, node, resource, showPanel, tab]);
};
