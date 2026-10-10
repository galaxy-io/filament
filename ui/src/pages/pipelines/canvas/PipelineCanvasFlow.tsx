import { type FC, useCallback, useMemo } from "react";

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
} from "@xyflow/react";

import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

import "@xyflow/react/dist/style.css";

import {
  PIPELINE_CANVAS_EDGE_TYPE,
  PIPELINE_CANVAS_OVERLAY_Z_INDEX,
  PIPELINE_CANVAS_PAN_ON_DRAG,
  PIPELINE_CANVAS_SNAP_GRID,
  PIPELINE_CANVAS_VIEW_SWITCHER_INSET,
} from "@/pages/pipelines/canvas/constants";
import PipelineCanvasEdge from "@/pages/pipelines/canvas/edges/PipelineCanvasEdge";
import { getPlaceholderNodes } from "@/pages/pipelines/canvas/graph/layout";
import { canConnectEdge } from "@/pages/pipelines/canvas/graph/rules";
import { usePipelineCanvasIsRunning } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasIsRunning";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import { usePipelineCanvasValidation } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasValidation";
import PipelineCanvasNodePlaceholder from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodePlaceholder";
import PipelineCanvasNodeSink from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeSink";
import PipelineCanvasNodeSource from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeSource";
import PipelineCanvasControls from "@/pages/pipelines/canvas/PipelineCanvasControls";
import PipelineCanvasEditWidget from "@/pages/pipelines/canvas/PipelineCanvasEditWidget";
import PipelineCanvasSelectionReveal from "@/pages/pipelines/canvas/PipelineCanvasSelectionReveal";
import PipelineCanvasViewSwitcher from "@/pages/pipelines/canvas/PipelineCanvasViewSwitcher";
import PipelineCanvasPanel from "@/pages/pipelines/canvas/panel/PipelineCanvasPanel";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";
import { PipelineCanvasNodeType } from "@/pages/pipelines/canvas/types";
import {
  getPipelineCanvasFitViewOptions,
  isConnectionNode,
  mapEdgesToStyledEdges,
  mapElementsToSelected,
} from "@/pages/pipelines/canvas/utils";

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

const FlowWrapper = styled.div`
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
    width: 114px;
    height: 72px;
    background-color: ${t.color.background.base};
    border: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
    border-radius: ${t.radius.lg};
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
  }
`;

const ViewSwitcherOverlay = styled.div`
  position: absolute;
  left: ${PIPELINE_CANVAS_VIEW_SWITCHER_INSET}px;
  top: ${PIPELINE_CANVAS_VIEW_SWITCHER_INSET}px;
  z-index: ${PIPELINE_CANVAS_OVERLAY_Z_INDEX};
`;

const PipelineCanvasFlow: FC = () => {
  const { theme } = useGalaxyTheme();
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

  const isRunning = usePipelineCanvasIsRunning();

  const { invalidEdgeIds } = usePipelineCanvasValidation();
  const styledEdges = useMemo(
    () => mapEdgesToStyledEdges(selectedEdges, selectedNodes, theme, isRunning, invalidEdgeIds),
    [selectedEdges, selectedNodes, theme, isRunning, invalidEdgeIds],
  );

  return (
    <FlowWrapper>
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
            return node.selected
              ? theme.color.solid.primary.background
              : theme.color.background.tertiary;
          }}
          nodeStrokeColor={(node) =>
            node.selected ? theme.color.solid.primary.background : theme.color.border.primary
          }
          nodeStrokeWidth={1}
          bgColor={theme.color.background.base}
          maskColor={`${theme.color.background.primary}80`}
          pannable
          zoomable
        />
      </ReactFlow>
      <ViewSwitcherOverlay>
        <PipelineCanvasViewSwitcher />
      </ViewSwitcherOverlay>
      {!isReadOnly && <PipelineCanvasEditWidget />}
      <PipelineCanvasPanel />
    </FlowWrapper>
  );
};

export default PipelineCanvasFlow;
