import type { FC } from "react";

import ToggleInput, {
  ToggleInputSize,
  ToggleInputVariant,
  type ToggleOption,
} from "@galaxy-io/dls/inputs/ToggleInput";

import {
  PIPELINE_CANVAS_VIEW_TO_ICON_MAP,
  PIPELINE_CANVAS_VIEW_TO_LABEL_MAP,
} from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import { PipelineCanvasView } from "@/pages/pipelines/canvas/types";

interface PipelineCanvasViewSwitcherProps {
  isIconOnly?: boolean;
}

const PipelineCanvasViewSwitcher: FC<PipelineCanvasViewSwitcherProps> = ({
  isIconOnly = false,
}) => {
  const { view, setView } = usePipelineCanvasSelection();

  const items: ToggleOption<PipelineCanvasView>[] = Object.values(PipelineCanvasView).map(
    (candidate) => {
      const label = PIPELINE_CANVAS_VIEW_TO_LABEL_MAP[candidate];
      return {
        id: candidate,
        icon: PIPELINE_CANVAS_VIEW_TO_ICON_MAP[candidate],
        ...(isIconOnly ? { ariaLabel: label } : { label }),
      };
    },
  );

  return (
    <ToggleInput
      ariaLabel="View"
      options={items}
      value={view}
      onChange={setView}
      size={ToggleInputSize.MEDIUM}
      variant={ToggleInputVariant.PRIMARY}
    />
  );
};

export default PipelineCanvasViewSwitcher;
