import type { FC } from "react";

import { XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Box from "@galaxy-io/dls/layout/Box";
import Tabs, { type TabItem, TabsSize } from "@galaxy-io/dls/navigation/Tabs";

import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import { PipelineCanvasPanelTab } from "@/pages/pipelines/canvas/panel/types";

const PIPELINE_CANVAS_PANEL_TAB_ITEMS: TabItem<PipelineCanvasPanelTab>[] = [
  { id: PipelineCanvasPanelTab.OVERVIEW, label: "Overview" },
  { id: PipelineCanvasPanelTab.ACTIVITY, label: "Activity" },
];

const PipelineCanvasPanelTabHeader: FC = () => {
  const { activeTab, setActiveTab, setShowPanel } = usePipelineCanvasSelection();

  return (
    <Box padding={[0, 12]} fillWidth>
      <Tabs
        ariaLabel="Configuration panel"
        size={TabsSize.MEDIUM}
        items={PIPELINE_CANVAS_PANEL_TAB_ITEMS}
        value={activeTab}
        onChange={setActiveTab}
        actions={
          <Button
            icon={XIcon}
            ariaLabel="Close configuration panel"
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
            onClick={() => setShowPanel(false)}
          />
        }
      />
    </Box>
  );
};

export default PipelineCanvasPanelTabHeader;
