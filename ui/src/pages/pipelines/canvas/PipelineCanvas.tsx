import { useMemo } from "react";

import { styled } from "@linaria/react";
import { Background, BackgroundVariant, MiniMap, ReactFlow } from "@xyflow/react";

import { useTheme, withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import "@xyflow/react/dist/style.css";

import { CANVAS_FIT_VIEW_OPTIONS, CANVAS_SNAP_GRID } from "@/pages/pipelines/canvas/constants";
import usePipelineCanvas from "@/pages/pipelines/canvas/hooks/usePipelineCanvas";
import PipelineNodeSink from "@/pages/pipelines/canvas/nodes/PipelineNodeSink";
import PipelineNodeSource from "@/pages/pipelines/canvas/nodes/PipelineNodeSource";
import PipelineCanvasControls from "@/pages/pipelines/canvas/PipelineCanvasControls";
import PipelineCanvasEditWidget from "@/pages/pipelines/canvas/PipelineCanvasEditWidget";
import { PipelineNodeType } from "@/pages/pipelines/canvas/types";

const pipelineNodeTypes = {
  [PipelineNodeType.SOURCE]: PipelineNodeSource,
  [PipelineNodeType.SINK]: PipelineNodeSink,
};

const CanvasWrapper = withTheme(styled.div<PropsWithTheme>`
  position: relative;
  width: 100%;
  height: 100%;

  .react-flow__edges {
    z-index: 1000 !important;
  }

  .react-flow__node.selected {
    z-index: 999 !important;
  }

  .react-flow__background {
    background-color: ${({ theme }) => theme.color.background.base};
  }

  .react-flow__minimap {
    position: absolute;
    bottom: 16px;
    left: 54px;
    margin: 0;
    width: 160px;
    height: 92px;
    background-color: ${({ theme }) => theme.color.background.base};
    border: 1px solid ${({ theme }) => theme.color.border.primary};
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

interface PipelineCanvasProps {
  pipelineId: string;
}

// biome-ignore lint/correctness/noUnusedFunctionParameters: pipelineId reserved for wiring saved pipeline state into the canvas (not implemented yet)
const PipelineCanvas = ({ pipelineId }: PipelineCanvasProps) => {
  const theme = useTheme();

  const { nodes, edges, onNodesChange, onEdgesChange, onConnect } = usePipelineCanvas();

  // Get IDs of selected nodes
  const selectedNodeIds = useMemo(
    () => new Set(nodes.filter((node) => node.selected).map((node) => node.id)),
    [nodes],
  );

  // Style edges based on selection state or connection to selected nodes
  const styledEdges = useMemo(
    () =>
      edges.map((edge) => {
        const isSelected = edge.selected;
        const isConnectedToSelected =
          selectedNodeIds.has(edge.source) || selectedNodeIds.has(edge.target);
        const isHighlighted = isSelected || isConnectedToSelected;

        return {
          ...edge,
          type: "bezier",
          animated: false,
          selectable: true,
          style: {
            stroke: isHighlighted ? theme.color.background.galaxy : theme.color.border.primary,
            strokeWidth: isSelected ? 3 : 2,
          },
        };
      }),
    [edges, selectedNodeIds, theme],
  );

  return (
    <CanvasWrapper>
      <ReactFlow
        nodes={nodes}
        edges={styledEdges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        nodeTypes={pipelineNodeTypes}
        snapToGrid
        snapGrid={CANVAS_SNAP_GRID}
        fitView
        fitViewOptions={CANVAS_FIT_VIEW_OPTIONS}
        deleteKeyCode={["Backspace", "Delete"]}
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
          nodeColor={(node) =>
            node.selected ? theme.color.background.galaxy : theme.color.background.tertiary
          }
          nodeStrokeColor={(node) =>
            node.selected ? theme.color.background.galaxy : theme.color.border.primary
          }
          nodeStrokeWidth={1}
          maskColor={`${theme.color.background.primary}80`}
          pannable
          zoomable
        />
      </ReactFlow>
      <PipelineCanvasEditWidget />
    </CanvasWrapper>
  );
};

export default PipelineCanvas;
