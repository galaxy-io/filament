import { useCallback, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { useParams } from "@tanstack/react-router";
import {
  Background,
  BackgroundVariant,
  type Connection,
  type EdgeChange,
  type EdgeTypes,
  MiniMap,
  type NodeChange,
  type NodeTypes,
  ReactFlow,
} from "@xyflow/react";

import { useTheme, withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";

import "@xyflow/react/dist/style.css";

import type { PropsWithTheme } from "@galaxy-io/dls/theme";

import { PaginationRequestSchema } from "@/gen/ingestion/v1/pagination_pb";
import { ListRunsRequestSchema } from "@/gen/ingestion/v1/runs_pb";

import {
  PIPELINE_CANVAS_EDGE_TYPE,
  PIPELINE_CANVAS_PAN_ON_DRAG,
  PIPELINE_CANVAS_SNAP_GRID,
} from "@/pages/pipelines/canvas/constants";
import PipelineCanvasEdge from "@/pages/pipelines/canvas/edges/PipelineCanvasEdge";
import { getPlaceholderNodes } from "@/pages/pipelines/canvas/graph/layout";
import { canConnectEdge } from "@/pages/pipelines/canvas/graph/rules";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasNodePlaceholder from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodePlaceholder";
import PipelineCanvasNodeSink from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeSink";
import PipelineCanvasNodeSource from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeSource";
import PipelineCanvasControls from "@/pages/pipelines/canvas/PipelineCanvasControls";
import PipelineCanvasEditWidget from "@/pages/pipelines/canvas/PipelineCanvasEditWidget";
import PipelineCanvasSelectionReveal from "@/pages/pipelines/canvas/PipelineCanvasSelectionReveal";
import PipelineCanvasPanel from "@/pages/pipelines/canvas/panel/PipelineCanvasPanel";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";
import { isConnectionNode, PipelineCanvasNodeType } from "@/pages/pipelines/canvas/types";
import {
  getPipelineCanvasFitViewOptions,
  mapEdgesToStyledEdges,
  mapElementsToSelected,
} from "@/pages/pipelines/canvas/utils";

import { ACTIVE_RUN_STATUSES } from "@/api/queries/constants";
import { useListRunsQuery } from "@/api/queries/runs";

const filterSelectionChanges = <T extends { type: string }>(changes: T[]): T[] =>
  changes.filter((change) => change.type !== "select");

const PIPELINE_CANVAS_NODE_TYPE_TO_COMPONENT_MAP: Record<
  PipelineCanvasNodeType,
  NodeTypes[string]
> = {
  [PipelineCanvasNodeType.SOURCE]: PipelineCanvasNodeSource,
  [PipelineCanvasNodeType.SINK]: PipelineCanvasNodeSink,
  [PipelineCanvasNodeType.PLACEHOLDER]: PipelineCanvasNodePlaceholder,
};

const PIPELINE_EDGE_TYPE_TO_COMPONENT_MAP: Record<
  typeof PIPELINE_CANVAS_EDGE_TYPE,
  EdgeTypes[string]
> = {
  [PIPELINE_CANVAS_EDGE_TYPE]: PipelineCanvasEdge,
};

const PipelineCanvasPageWrapper = withTheme(styled.div<PropsWithTheme>`
  position: relative;
  flex: 1;
  min-width: 0;
  height: 100%;

  .react-flow__pane {
    cursor: grab;
  }

  .react-flow__pane.dragging {
    cursor: grabbing;
  }

  .react-flow__minimap {
    position: absolute;
    bottom: 16px;
    left: 54px;
    margin: 0;
    width: 160px;
    height: 92px;
    background-color: ${({ theme }) => theme.color.background.base};
    border: 0.5px solid ${({ theme }) => theme.color.border.primary};
    border-radius: 6px;
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
  }
`);

const PipelineCanvasPage = () => {
  const theme = useTheme();
  const { id } = useParams({ from: "/_app/pipelines/$id" });
  const state = usePipelineCanvasState();
  const { applyNodeChanges, applyEdgeChanges, connect } = usePipelineCanvasActions();
  const isReadOnly = usePipelineCanvasReadOnly();
  const {
    selectedNodeId,
    selectedResourceId,
    showPanel,
    selectNode,
    selectResource,
    clearSelection,
  } = usePipelineCanvasSelection();

  const onNodesChange = useCallback(
    (changes: NodeChange<CanvasNode>[]) => {
      if (isReadOnly) return;
      const applicable = filterSelectionChanges(changes);
      if (applicable.length) applyNodeChanges(applicable);
    },
    [applyNodeChanges, isReadOnly],
  );

  const onEdgesChange = useCallback(
    (changes: EdgeChange<CanvasEdge>[]) => {
      if (isReadOnly) return;
      const applicable = filterSelectionChanges(changes);
      if (applicable.length) applyEdgeChanges(applicable);
    },
    [applyEdgeChanges, isReadOnly],
  );

  const onNodeClick = useCallback(
    (_event: React.MouseEvent, node: CanvasNode) => {
      if (!isConnectionNode(node)) return;
      selectNode(node.id);
    },
    [selectNode],
  );

  const onEdgeClick = useCallback(
    (_event: React.MouseEvent, edge: CanvasEdge) => selectResource(edge.id),
    [selectResource],
  );

  const onConnect = useCallback(
    (connection: Connection) => {
      if (isReadOnly) return;
      connect(connection);
    },
    [connect, isReadOnly],
  );

  const isValidConnection = useCallback(
    (connection: Connection | CanvasEdge) => canConnectEdge(connection, state.edges),
    [state.edges],
  );

  const selectedNodes = useMemo(
    () => mapElementsToSelected(state.nodes, selectedNodeId),
    [state.nodes, selectedNodeId],
  );

  const selectedEdges = useMemo(
    () => mapElementsToSelected(state.edges, selectedResourceId),
    [state.edges, selectedResourceId],
  );

  const renderedNodes = useMemo(
    () => [...selectedNodes, ...getPlaceholderNodes(state.nodes, isReadOnly)],
    [selectedNodes, state.nodes, isReadOnly],
  );

  const fitViewOptions = useMemo(() => getPipelineCanvasFitViewOptions(showPanel), [showPanel]);

  const { data: activeRunsData } = useListRunsQuery({
    input: create(ListRunsRequestSchema, {
      pipelineId: id,
      status: [...ACTIVE_RUN_STATUSES],
      pagination: create(PaginationRequestSchema, { pageSize: 1 }),
    }),
  });
  const isRunning = (activeRunsData?.runs.length ?? 0) > 0;

  const styledEdges = useMemo(
    () => mapEdgesToStyledEdges(selectedEdges, selectedNodes, theme, isRunning),
    [selectedEdges, selectedNodes, theme, isRunning],
  );

  return (
    <PipelineCanvasPageWrapper>
      <ReactFlow
        nodes={renderedNodes}
        edges={styledEdges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        isValidConnection={isValidConnection}
        onNodeClick={onNodeClick}
        onEdgeClick={onEdgeClick}
        onPaneClick={clearSelection}
        nodeTypes={PIPELINE_CANVAS_NODE_TYPE_TO_COMPONENT_MAP}
        edgeTypes={PIPELINE_EDGE_TYPE_TO_COMPONENT_MAP}
        nodesDraggable={!isReadOnly}
        nodesConnectable={!isReadOnly}
        elementsSelectable
        panOnDrag={PIPELINE_CANVAS_PAN_ON_DRAG}
        panOnScroll
        snapToGrid
        snapGrid={PIPELINE_CANVAS_SNAP_GRID}
        fitView
        fitViewOptions={fitViewOptions}
        deleteKeyCode={isReadOnly ? null : ["Backspace", "Delete"]}
        proOptions={{ hideAttribution: true }}
      >
        <Background
          variant={BackgroundVariant.Dots}
          gap={PIPELINE_CANVAS_SNAP_GRID[0]}
          size={1}
          color={theme.color.border.primary}
          bgColor={theme.color.background.base}
        />
        <PipelineCanvasControls />
        <PipelineCanvasSelectionReveal />
        <MiniMap
          nodeColor={(node) => {
            if (node.type === PipelineCanvasNodeType.PLACEHOLDER) return "transparent";
            return node.selected ? theme.color.background.galaxy : theme.color.background.tertiary;
          }}
          nodeStrokeColor={(node) =>
            node.selected ? theme.color.background.galaxy : theme.color.border.primary
          }
          nodeStrokeWidth={1}
          bgColor={theme.color.background.base}
          maskColor={`${theme.color.background.primary}80`}
          pannable
          zoomable
        />
      </ReactFlow>
      {!isReadOnly && <PipelineCanvasEditWidget />}
      <PipelineCanvasPanel />
    </PipelineCanvasPageWrapper>
  );
};

export default PipelineCanvasPage;
