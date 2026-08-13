import type { CanvasNode } from "@/pages/pipelines/canvas/types";

export interface PipelineCanvasNodeIslandMeasurements {
  listTop: number;
  listBottom: number;
  badgeAnchorX: number | null;
  badgeAnchorY: number | null;
  hiddenHandleIds: string[];
}

const pipelineCanvasNodeMeasurementsMap = new Map<
  CanvasNode["id"],
  PipelineCanvasNodeIslandMeasurements
>();
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
  a.listTop === b.listTop &&
  a.listBottom === b.listBottom &&
  a.badgeAnchorX === b.badgeAnchorX &&
  a.badgeAnchorY === b.badgeAnchorY &&
  a.hiddenHandleIds.length === b.hiddenHandleIds.length &&
  a.hiddenHandleIds.every((id, index) => id === b.hiddenHandleIds[index]);

export const setPipelineCanvasNodeMeasurements = (
  nodeId: CanvasNode["id"],
  next: PipelineCanvasNodeIslandMeasurements,
) => {
  // Bailing on equal values keeps the stored object identity stable, which is what
  // lets useSyncExternalStore consumers cache a snapshot across redundant publishes.
  const prev = pipelineCanvasNodeMeasurementsMap.get(nodeId);
  if (prev && isSameMeasurements(prev, next)) return;

  pipelineCanvasNodeMeasurementsMap.set(nodeId, next);
  notifyPipelineCanvasNodeMeasurementListeners();
};

export const removePipelineCanvasNodeMeasurements = (nodeId: CanvasNode["id"]) => {
  if (!pipelineCanvasNodeMeasurementsMap.delete(nodeId)) return;

  notifyPipelineCanvasNodeMeasurementListeners();
};

export const subscribePipelineCanvasNodeMeasurements = (listener: () => void) => {
  pipelineCanvasNodeMeasurementListeners.add(listener);
  return () => {
    pipelineCanvasNodeMeasurementListeners.delete(listener);
  };
};

export const getPipelineCanvasNodeMeasurements = (nodeId: CanvasNode["id"]) =>
  pipelineCanvasNodeMeasurementsMap.get(nodeId);
