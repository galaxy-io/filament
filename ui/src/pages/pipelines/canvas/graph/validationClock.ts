import { PIPELINE_CANVAS_VALIDATION_DEBOUNCE_MS } from "@/pages/pipelines/canvas/constants";

let pipelineCanvasValidationTick = 0;
let pipelineCanvasValidationTimer: ReturnType<typeof setTimeout> | null = null;
const pipelineCanvasValidationListeners = new Set<() => void>();

const clearPipelineCanvasValidationTimer = () => {
  if (pipelineCanvasValidationTimer === null) return;
  clearTimeout(pipelineCanvasValidationTimer);
  pipelineCanvasValidationTimer = null;
};

export const pokePipelineCanvasValidation = () => {
  clearPipelineCanvasValidationTimer();
  pipelineCanvasValidationTimer = setTimeout(() => {
    pipelineCanvasValidationTimer = null;
    pipelineCanvasValidationTick += 1;
    for (const listener of pipelineCanvasValidationListeners) {
      listener();
    }
  }, PIPELINE_CANVAS_VALIDATION_DEBOUNCE_MS);
};

export const subscribePipelineCanvasValidation = (listener: () => void) => {
  pipelineCanvasValidationListeners.add(listener);
  return () => {
    pipelineCanvasValidationListeners.delete(listener);
    if (pipelineCanvasValidationListeners.size === 0) {
      clearPipelineCanvasValidationTimer();
    }
  };
};

export const getPipelineCanvasValidationTick = () => pipelineCanvasValidationTick;
