export interface PipelineCanvasNodeIslandMeasurements {
  bodyTop: number;
  bodyBottom: number;
  badgeAnchorX: number | null;
  badgeAnchorY: number | null;
  hiddenHandleIds: string[];
}

const pipelineCanvasNodeMeasurementsMap = new Map<string, PipelineCanvasNodeIslandMeasurements>();
const pipelineCanvasNodeMeasurementListeners = new Set<() => void>();

const notifyPipelineCanvasNodeMeasurementListeners = () => {
  for (const listener of pipelineCanvasNodeMeasurementListeners) {
    listener();
  }
};

const isSameMeasurements = (
  a: PipelineCanvasNodeIslandMeasurements,
  b: PipelineCanvasNodeIslandMeasurements,
) =>
  a.bodyTop === b.bodyTop &&
  a.bodyBottom === b.bodyBottom &&
  a.badgeAnchorX === b.badgeAnchorX &&
  a.badgeAnchorY === b.badgeAnchorY &&
  a.hiddenHandleIds.length === b.hiddenHandleIds.length &&
  a.hiddenHandleIds.every((id, index) => id === b.hiddenHandleIds[index]);

export const setPipelineCanvasNodeMeasurements = (
  nodeId: string,
  next: PipelineCanvasNodeIslandMeasurements,
) => {
  const prev = pipelineCanvasNodeMeasurementsMap.get(nodeId);
  if (prev && isSameMeasurements(prev, next)) return;
  pipelineCanvasNodeMeasurementsMap.set(nodeId, next);
  notifyPipelineCanvasNodeMeasurementListeners();
};

export const removePipelineCanvasNodeMeasurements = (nodeId: string) => {
  if (pipelineCanvasNodeMeasurementsMap.delete(nodeId)) {
    notifyPipelineCanvasNodeMeasurementListeners();
  }
};

export const subscribePipelineCanvasNodeMeasurements = (listener: () => void) => {
  pipelineCanvasNodeMeasurementListeners.add(listener);
  return () => pipelineCanvasNodeMeasurementListeners.delete(listener);
};

export const getPipelineCanvasNodeMeasurements = (nodeId: string) =>
  pipelineCanvasNodeMeasurementsMap.get(nodeId);
