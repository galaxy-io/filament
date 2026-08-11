import { useCallback, useMemo } from "react";

import { styled } from "@linaria/react";
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
  SelectionMode,
} from "@xyflow/react";

import { useTheme, withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";

import "@xyflow/react/dist/style.css";

import type { PropsWithTheme } from "@galaxy-io/dls/theme";

import {
  PIPELINE_CANVAS_EDGE_TYPE,
  PIPELINE_CANVAS_FIT_VIEW_OPTIONS,
  PIPELINE_CANVAS_SNAP_GRID,
} from "@/pages/pipelines/canvas/constants";
import PipelineCanvasEdge from "@/pages/pipelines/canvas/edges/PipelineCanvasEdge";
import { getPlaceholderNodes } from "@/pages/pipelines/canvas/graph/layout";
import PipelineCanvasNodePlaceholder from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodePlaceholder";
import PipelineCanvasNodeSink from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeSink";
import PipelineCanvasNodeSource from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeSource";
import PipelineCanvasControls from "@/pages/pipelines/canvas/PipelineCanvasControls";
import PipelineCanvasEditWidget from "@/pages/pipelines/canvas/PipelineCanvasEditWidget";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { PipelineCanvasInteractionMode } from "@/pages/pipelines/canvas/providers/canvas/types";
import PipelineCanvasTerminal from "@/pages/pipelines/canvas/terminal/PipelineCanvasTerminal";
import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";
import { PipelineCanvasNodeType } from "@/pages/pipelines/canvas/types";
import { mapEdgesToStyledEdges } from "@/pages/pipelines/canvas/utils";

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

const PipelineCanvasPageWrapper = withTheme(styled.div<PropsWithTheme<{ $isGrabMode?: boolean }>>`
  position: relative;
  flex: 1;
  min-width: 0;
  height: 100%;

  .react-flow__selection {
    background: color-mix(
      in srgb,
      ${({ theme }) => theme.color.background.galaxy} 8%,
      transparent
    );
    border: 1px solid ${({ theme }) => theme.color.background.galaxy};
    border-radius: 6px;
  }

  .react-flow__nodesselection-rect {
    background: transparent;
    border: none;
  }

  .react-flow__pane {
    cursor: ${({ $isGrabMode }) => ($isGrabMode ? "grab" : "default")};
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
  const state = usePipelineCanvasState();
  const { applyNodeChanges, applyEdgeChanges, connect } = usePipelineCanvasActions();
  const isReadOnly = usePipelineCanvasReadOnly();

  const isGrabMode = state.interactionMode === PipelineCanvasInteractionMode.GRAB;

  const onNodesChange = useCallback(
    (changes: NodeChange<CanvasNode>[]) => {
      if (isReadOnly) return;
      applyNodeChanges(changes);
    },
    [applyNodeChanges, isReadOnly],
  );

  const onEdgesChange = useCallback(
    (changes: EdgeChange<CanvasEdge>[]) => {
      if (isReadOnly) return;
      applyEdgeChanges(changes);
    },
    [applyEdgeChanges, isReadOnly],
  );

  const onConnect = useCallback(
    (connection: Connection) => {
      if (isReadOnly) return;
      connect(connection);
    },
    [connect, isReadOnly],
  );

  const renderedNodes = useMemo(
    () => [...state.nodes, ...getPlaceholderNodes(state.nodes, isReadOnly)],
    [state.nodes, isReadOnly],
  );

  const styledEdges = useMemo(
    () => mapEdgesToStyledEdges(state.edges, state.nodes, theme),
    [state.edges, state.nodes, theme],
  );

  return (
    <PipelineCanvasPageWrapper $isGrabMode={isGrabMode}>
      <ReactFlow
        nodes={renderedNodes}
        edges={styledEdges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        nodeTypes={PIPELINE_CANVAS_NODE_TYPE_TO_COMPONENT_MAP}
        edgeTypes={PIPELINE_EDGE_TYPE_TO_COMPONENT_MAP}
        nodesDraggable={!isReadOnly}
        nodesConnectable={!isReadOnly}
        elementsSelectable={!isReadOnly}
        selectionOnDrag={!isReadOnly && !isGrabMode}
        selectionMode={SelectionMode.Partial}
        panOnDrag={isGrabMode ? [0, 1, 2] : [1, 2]}
        panOnScroll
        snapToGrid
        snapGrid={PIPELINE_CANVAS_SNAP_GRID}
        fitView
        fitViewOptions={PIPELINE_CANVAS_FIT_VIEW_OPTIONS}
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
      {!isReadOnly && <PipelineCanvasTerminal />}
    </PipelineCanvasPageWrapper>
  );
};

export default PipelineCanvasPage;
