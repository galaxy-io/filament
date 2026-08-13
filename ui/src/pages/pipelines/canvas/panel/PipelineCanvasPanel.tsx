import { styled } from "@linaria/react";
import { SidebarSimpleIcon } from "@phosphor-icons/react";
import { match } from "ts-pattern";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { PIPELINE_CANVAS_OVERLAY_Z_INDEX } from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasPanelActivity from "@/pages/pipelines/canvas/panel/activity/PipelineCanvasPanelActivity";
import {
  PIPELINE_CANVAS_PANEL_INSET,
  PIPELINE_CANVAS_PANEL_WIDTH,
} from "@/pages/pipelines/canvas/panel/constants";
import PipelineCanvasPanelOverview from "@/pages/pipelines/canvas/panel/overview/PipelineCanvasPanelOverview";
import PipelineCanvasPanelNodeDetail from "@/pages/pipelines/canvas/panel/overview/resource/PipelineCanvasPanelNodeDetail";
import PipelineCanvasPanelResourceDetail from "@/pages/pipelines/canvas/panel/overview/resource/PipelineCanvasPanelResourceDetail";
import PipelineCanvasPanelTabHeader from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelTabHeader";
import { PipelineCanvasPanelTab } from "@/pages/pipelines/canvas/panel/types";
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { isConnectionNode } from "@/pages/pipelines/canvas/types";

const PanelWrapper = withTheme(styled.div<PropsWithTheme>`
  position: absolute;
  top: ${PIPELINE_CANVAS_PANEL_INSET}px;
  right: ${PIPELINE_CANVAS_PANEL_INSET}px;
  bottom: ${PIPELINE_CANVAS_PANEL_INSET}px;
  z-index: ${PIPELINE_CANVAS_OVERLAY_Z_INDEX};

  width: ${PIPELINE_CANVAS_PANEL_WIDTH}px;

  display: flex;
  flex-direction: column;

  background-color: ${({ theme }) => theme.color.background.primary};

  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 6px;

  overflow: hidden;
`);

const CollapsedWrapper = styled.div`
  position: absolute;
  top: ${PIPELINE_CANVAS_PANEL_INSET}px;
  right: ${PIPELINE_CANVAS_PANEL_INSET}px;
  z-index: ${PIPELINE_CANVAS_OVERLAY_Z_INDEX};

  padding: 8px;

  border: 0.5px solid transparent;
`;

const PipelineCanvasPanel = () => {
  const state = usePipelineCanvasState();
  const { selectedNodeId, selectedResourceId, showPanel, activeTab, setShowPanel } =
    usePipelineCanvasSelection();

  if (!showPanel) {
    return (
      <CollapsedWrapper>
        <Button
          icon={SidebarSimpleIcon}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.SMALL}
          onClick={() => setShowPanel(true)}
          ariaLabel="Open configuration panel"
        />
      </CollapsedWrapper>
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

  return <PanelWrapper>{renderContent()}</PanelWrapper>;
};

export default PipelineCanvasPanel;
