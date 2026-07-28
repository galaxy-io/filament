export interface PipelineNodeIslandMeasurements {
  listTop: number;
  listBottom: number;
  badgeCenterY: number | null;
}

const pipelineNodeMeasurementsMap = new Map<string, PipelineNodeIslandMeasurements>();
const pipelineNodeMeasurementListeners = new Set<() => void>();

const notifyPipelineNodeMeasurementListeners = () => {
  for (const listener of pipelineNodeMeasurementListeners) {
    listener();
  }
};

const isSameMeasurements = (a: PipelineNodeIslandMeasurements, b: PipelineNodeIslandMeasurements) =>
  a.listTop === b.listTop && a.listBottom === b.listBottom && a.badgeCenterY === b.badgeCenterY;

export const setPipelineNodeMeasurements = (
  nodeId: string,
  next: PipelineNodeIslandMeasurements,
) => {
  const prev = pipelineNodeMeasurementsMap.get(nodeId);
  if (prev && isSameMeasurements(prev, next)) return;
  pipelineNodeMeasurementsMap.set(nodeId, next);
  notifyPipelineNodeMeasurementListeners();
};

export const removePipelineNodeMeasurements = (nodeId: string) => {
  if (pipelineNodeMeasurementsMap.delete(nodeId)) {
    notifyPipelineNodeMeasurementListeners();
  }
};

export const subscribePipelineNodeMeasurements = (listener: () => void) => {
  pipelineNodeMeasurementListeners.add(listener);
  return () => pipelineNodeMeasurementListeners.delete(listener);
};

export const getPipelineNodeMeasurements = (nodeId: string) =>
  pipelineNodeMeasurementsMap.get(nodeId);
