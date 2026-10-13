import type { FC } from "react";

import { SidebarSimpleIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";

const PipelineCanvasPanelOpenButton: FC = () => {
  const { showPanel, setShowPanel } = usePipelineCanvasSelection();

  if (showPanel) {
    return null;
  }

  return (
    <Button
      icon={SidebarSimpleIcon}
      variant={ButtonVariant.SECONDARY}
      size={ButtonSize.MEDIUM}
      ariaLabel="Open configuration panel"
      onClick={() => setShowPanel(true)}
    />
  );
};

export default PipelineCanvasPanelOpenButton;
