import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SwitcherInput, { type SwitcherInputItem } from "@galaxy-io/dls/inputs/SwitcherInput";

import {
  PIPELINE_CANVAS_VIEW_TO_ICON_MAP,
  PIPELINE_CANVAS_VIEW_TO_LABEL_MAP,
} from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import { PipelineCanvasView } from "@/pages/pipelines/canvas/types";

const PipelineCanvasViewSwitcher = () => {
  const { view, setView } = usePipelineCanvasSelection();

  const items: SwitcherInputItem[] = Object.values(PipelineCanvasView).map((candidate) => ({
    id: candidate,
    label: PIPELINE_CANVAS_VIEW_TO_LABEL_MAP[candidate],
    icon: PIPELINE_CANVAS_VIEW_TO_ICON_MAP[candidate],
    onClick: () => setView(candidate),
  }));

  return (
    <SwitcherInput
      items={items}
      selectedId={view}
      size={InputSize.MEDIUM}
      variant={InputVariant.TERTIARY}
    />
  );
};

export default PipelineCanvasViewSwitcher;
