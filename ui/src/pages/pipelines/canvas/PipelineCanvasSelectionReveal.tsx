import type { FC } from "react";

import { usePipelineCanvasSelectionReveal } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelectionReveal";

const PipelineCanvasSelectionReveal: FC = () => {
  usePipelineCanvasSelectionReveal();
  return null;
};

export default PipelineCanvasSelectionReveal;
