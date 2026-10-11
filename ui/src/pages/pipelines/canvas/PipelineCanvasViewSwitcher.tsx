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

const PipelineCanvasViewSwitcher: FC = () => {
  const { view, setView } = usePipelineCanvasSelection();

  const items: ToggleOption<PipelineCanvasView>[] = Object.values(PipelineCanvasView).map(
    (candidate) => ({
      id: candidate,
      label: PIPELINE_CANVAS_VIEW_TO_LABEL_MAP[candidate],
      icon: PIPELINE_CANVAS_VIEW_TO_ICON_MAP[candidate],
    }),
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
