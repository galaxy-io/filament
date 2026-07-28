import { useCallback, useMemo } from "react";

import { styled } from "@linaria/react";
import {
  Background,
  BackgroundVariant,
  type Connection,
  type EdgeChange,
  MiniMap,
  type NodeChange,
  ReactFlow,
  SelectionMode,
} from "@xyflow/react";

import { useTheme, withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import "@xyflow/react/dist/style.css";

import { PipelineCanvasActionType } from "@/pages/pipelines/canvas/actions";
import {
  PIPELINE_CANVAS_EDGE_TYPE,
  PIPELINE_CANVAS_FIT_VIEW_OPTIONS,
  PIPELINE_CANVAS_SNAP_GRID,
} from "@/pages/pipelines/canvas/constants";
import PipelineCanvasEdge from "@/pages/pipelines/canvas/edges/PipelineCanvasEdge";
import { getPlaceholderNodes } from "@/pages/pipelines/canvas/graph";
import PipelineNodePlaceholder from "@/pages/pipelines/canvas/nodes/PipelineNodePlaceholder";
import PipelineNodeSink from "@/pages/pipelines/canvas/nodes/PipelineNodeSink";
import PipelineNodeSource from "@/pages/pipelines/canvas/nodes/PipelineNodeSource";
import PipelineCanvasControls from "@/pages/pipelines/canvas/PipelineCanvasControls";
import PipelineCanvasEditWidget from "@/pages/pipelines/canvas/PipelineCanvasEditWidget";
import { usePipelineCanvas } from "@/pages/pipelines/canvas/PipelineCanvasProvider";
import PipelineCanvasTerminal from "@/pages/pipelines/canvas/terminal/PipelineCanvasTerminal";
import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";
import { PipelineCanvasInteractionMode, PipelineNodeType } from "@/pages/pipelines/canvas/types";

const pipelineNodeTypes = {
  [PipelineNodeType.SOURCE]: PipelineNodeSource,
  [PipelineNodeType.SINK]: PipelineNodeSink,
  [PipelineNodeType.PLACEHOLDER]: PipelineNodePlaceholder,
};

const pipelineEdgeTypes = {
  [PIPELINE_CANVAS_EDGE_TYPE]: PipelineCanvasEdge,
};

const PageWrapper = styled.div`
  width: 100%;
  height: 100%;

  display: flex;
`;

const CanvasWrapper = withTheme(styled.div<PropsWithTheme<{ $isGrabMode?: boolean }>>`
  position: relative;
  flex: 1;
  min-width: 0;
  height: 100%;

  .react-flow__edges {
    z-index: 1000 !important;
  }

  .react-flow__node.selected {
    z-index: 999 !important;
  }

  /* Placeholder ghosts are non-draggable/selectable/connectable, which makes
     React Flow drop their pointer events - restore them so clicks expand */
  .react-flow__node-PLACEHOLDER {
    pointer-events: all !important;
  }

  .react-flow__background {
    background-color: ${({ theme }) => theme.color.background.base};
  }

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

    svg {
      background-color: ${({ theme }) => theme.color.background.base};
    }
  }
`);

const PipelineCanvas = () => {
  const theme = useTheme();
  const { state, dispatch } = usePipelineCanvas();

  const isGrabMode = state.interactionMode === PipelineCanvasInteractionMode.GRAB;
  const isReadOnly = state.isReadOnly;

  const onNodesChange = useCallback(
    (changes: NodeChange<CanvasNode>[]) => {
      if (isReadOnly) return;
      dispatch({
        type: PipelineCanvasActionType.APPLY_NODE_CHANGES,
        payload: changes,
      });
    },
    [dispatch, isReadOnly],
  );

  const onEdgesChange = useCallback(
    (changes: EdgeChange<CanvasEdge>[]) => {
      if (isReadOnly) return;
      dispatch({
        type: PipelineCanvasActionType.APPLY_EDGE_CHANGES,
        payload: changes,
      });
    },
    [dispatch, isReadOnly],
  );

  const onConnect = useCallback(
    (connection: Connection) => {
      if (isReadOnly) return;
      dispatch({
        type: PipelineCanvasActionType.CONNECT,
        payload: connection,
      });
    },
    [dispatch, isReadOnly],
  );

  const renderedNodes = useMemo(
    () => [...state.nodes, ...getPlaceholderNodes(state.nodes, isReadOnly)],
    [state.nodes, isReadOnly],
  );

  const selectedNodeIds = useMemo(
    () => new Set(state.nodes.filter((node) => node.selected).map((node) => node.id)),
    [state.nodes],
  );

  const styledEdges = useMemo<CanvasEdge[]>(
    () =>
      state.edges.map((edge) => {
        const isConnectedToSelected =
          selectedNodeIds.has(edge.source) || selectedNodeIds.has(edge.target);
        const isHighlighted = edge.selected || isConnectedToSelected;

        return {
          ...edge,
          style: {
            stroke: isHighlighted ? theme.color.background.galaxy : theme.color.border.primary,
            strokeWidth: edge.selected ? 3 : 2,
          },
        };
      }),
    [state.edges, selectedNodeIds, theme],
  );

  return (
    <PageWrapper>
      <CanvasWrapper $isGrabMode={isGrabMode}>
        <ReactFlow
          nodes={renderedNodes}
          edges={styledEdges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onConnect={onConnect}
          nodeTypes={pipelineNodeTypes}
          edgeTypes={pipelineEdgeTypes}
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
            gap={20}
            size={1}
            color={theme.color.border.primary}
          />
          <PipelineCanvasControls />
          <MiniMap
            nodeColor={(node) => {
              if (node.type === PipelineNodeType.PLACEHOLDER) return "transparent";
              return node.selected
                ? theme.color.background.galaxy
                : theme.color.background.tertiary;
            }}
            nodeStrokeColor={(node) =>
              node.selected ? theme.color.background.galaxy : theme.color.border.primary
            }
            nodeStrokeWidth={1}
            maskColor={`${theme.color.background.primary}80`}
            pannable
            zoomable
          />
        </ReactFlow>
        {!isReadOnly && <PipelineCanvasEditWidget />}
        {!isReadOnly && <PipelineCanvasTerminal />}
      </CanvasWrapper>
    </PageWrapper>
  );
};

export default PipelineCanvas;
