import { useMemo } from "react";

import { PipelineCanvasPanelTab } from "@/pages/pipelines/canvas/panel/types";
import {
  type CanvasEdge,
  type CanvasNode,
  PipelineCanvasView,
} from "@/pages/pipelines/canvas/types";

import { useFilamentSearchUpdate, usePipelineCanvasSearch } from "@/module/hooks";
import type { PipelineCanvasSearch } from "@/module/schemas";

export const usePipelineCanvasSelection = () => {
  const { node, resource, showPanel, tab, view, sinks } = usePipelineCanvasSearch();
  const updateSearch = useFilamentSearchUpdate<PipelineCanvasSearch>();

  return useMemo(() => {
    const setSearch = (patch: PipelineCanvasSearch, replace = true) =>
      void updateSearch((prev) => ({ ...prev, ...patch }), { replace });

    return {
      selectedNodeId: node,
      selectedResourceId: resource,
      showPanel: showPanel ?? false,
      activeTab: tab ?? PipelineCanvasPanelTab.OVERVIEW,
      view: view ?? PipelineCanvasView.CANVAS,
      sinkIds: sinks ?? [],
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
      setView: (nextView: PipelineCanvasView) =>
        setSearch({ view: nextView === PipelineCanvasView.CANVAS ? undefined : nextView }, false),
      setSinkIds: (nextSinkIds: CanvasNode["id"][]) =>
        setSearch({ sinks: nextSinkIds.length ? nextSinkIds : undefined }),
    };
  }, [updateSearch, node, resource, showPanel, tab, view, sinks]);
};
