import type { FC } from "react";

import { SidebarSimpleIcon } from "@phosphor-icons/react";
import { match } from "ts-pattern";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import { Radius } from "@galaxy-io/dls/theme/enums";

import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasPanelActivity from "@/pages/pipelines/canvas/panel/activity/PipelineCanvasPanelActivity";
import {
  PIPELINE_CANVAS_PANEL_INSET,
  PIPELINE_CANVAS_PANEL_WIDTH,
} from "@/pages/pipelines/canvas/panel/constants";
import PipelineCanvasPanelNodeDetail from "@/pages/pipelines/canvas/panel/overview/node/PipelineCanvasPanelNodeDetail";
import PipelineCanvasPanelOverview from "@/pages/pipelines/canvas/panel/overview/PipelineCanvasPanelOverview";
import PipelineCanvasPanelResourceDetail from "@/pages/pipelines/canvas/panel/overview/resource/PipelineCanvasPanelResourceDetail";
import PipelineCanvasPanelTabHeader from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelTabHeader";
import { PipelineCanvasPanelTab } from "@/pages/pipelines/canvas/panel/types";
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { isConnectionNode } from "@/pages/pipelines/canvas/utils";

const PipelineCanvasPanel: FC = () => {
  const state = usePipelineCanvasState();
  const { selectedNodeId, selectedResourceId, showPanel, activeTab, setShowPanel } =
    usePipelineCanvasSelection();

  if (!showPanel) {
    return (
      <Box
        position="absolute"
        inset={{ top: PIPELINE_CANVAS_PANEL_INSET, right: PIPELINE_CANVAS_PANEL_INSET }}
        padding={[8, 12]}
      >
        <Button
          icon={SidebarSimpleIcon}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.SMALL}
          onClick={() => setShowPanel(true)}
          ariaLabel="Open configuration panel"
        />
      </Box>
    );
  }

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
        <PipelineCanvasPanelTabHeader />
        {match(activeTab)
          .with(PipelineCanvasPanelTab.ACTIVITY, () => <PipelineCanvasPanelActivity />)
          .with(PipelineCanvasPanelTab.OVERVIEW, () => <PipelineCanvasPanelOverview />)
          .exhaustive()}
      </>
    );
  };

  return (
    <Box
      position="absolute"
      inset={{
        top: PIPELINE_CANVAS_PANEL_INSET,
        right: PIPELINE_CANVAS_PANEL_INSET,
        bottom: PIPELINE_CANVAS_PANEL_INSET,
      }}
      width={PIPELINE_CANVAS_PANEL_WIDTH}
      variant={BoxVariant.PRIMARY}
      hasBorder
      radius={Radius.LG}
      overflow="hidden"
    >
      <Flex direction={FlexDirection.COLUMN} height="100%">
        {renderContent()}
      </Flex>
    </Box>
  );
};

export default PipelineCanvasPanel;
