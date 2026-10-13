import type { FC } from "react";

import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";

import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasPanelNodeDetail from "@/pages/pipelines/canvas/panel/overview/node/PipelineCanvasPanelNodeDetail";
import PipelineCanvasPanelOverview from "@/pages/pipelines/canvas/panel/overview/PipelineCanvasPanelOverview";
import PipelineCanvasPanelResourceDetail from "@/pages/pipelines/canvas/panel/overview/resource/PipelineCanvasPanelResourceDetail";
import PipelineCanvasPanelHeader from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelHeader";
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { isConnectionNode } from "@/pages/pipelines/canvas/utils";

const PipelineCanvasPanel: FC = () => {
  const state = usePipelineCanvasState();
  const { selectedNodeId, selectedResourceId, setShowPanel } = usePipelineCanvasSelection();

  const selectedNode = state.nodes
    .filter(isConnectionNode)
    .find((node) => node.id === selectedNodeId);
  const selectedEdge = state.edges.find((edge) => edge.id === selectedResourceId);

  const renderContent = () => {
    if (selectedNode) {
      return <PipelineCanvasPanelNodeDetail key={selectedNode.id} node={selectedNode} />;
    }
    if (selectedEdge) {
      return <PipelineCanvasPanelResourceDetail key={selectedEdge.id} edge={selectedEdge} />;
    }
    return (
      <>
        <PipelineCanvasPanelHeader
          title="Overview"
          onClose={() => setShowPanel(false)}
          hasDivider
        />
        <PipelineCanvasPanelOverview />
      </>
    );
  };

  return (
    <Box variant={BoxVariant.PRIMARY} height="100%">
      <Flex direction={FlexDirection.COLUMN} height="100%">
        {renderContent()}
      </Flex>
    </Box>
  );
};

export default PipelineCanvasPanel;
