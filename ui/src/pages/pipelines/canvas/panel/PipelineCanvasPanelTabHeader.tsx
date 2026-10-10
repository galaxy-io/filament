import type { FC } from "react";

import { XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Tabs, { type TabItem, TabsSize } from "@galaxy-io/dls/navigation/Tabs";

import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import { PIPELINE_CANVAS_PANEL_TABS_INSET } from "@/pages/pipelines/canvas/panel/constants";
import { PipelineCanvasPanelTab } from "@/pages/pipelines/canvas/panel/types";

const PIPELINE_CANVAS_PANEL_TAB_ITEMS: TabItem<PipelineCanvasPanelTab>[] = [
  { id: PipelineCanvasPanelTab.OVERVIEW, label: "Overview" },
  { id: PipelineCanvasPanelTab.ACTIVITY, label: "Activity" },
];

const PipelineCanvasPanelTabHeader: FC = () => {
  const { activeTab, setActiveTab, setShowPanel } = usePipelineCanvasSelection();

  return (
    <Tabs
      ariaLabel="Configuration panel"
      inset={PIPELINE_CANVAS_PANEL_TABS_INSET}
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
  );
};

export default PipelineCanvasPanelTabHeader;
