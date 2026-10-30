import { usePipelineCanvasActiveRuns } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasActiveRuns";

export const usePipelineCanvasIsRunning = () => usePipelineCanvasActiveRuns().length > 0;
