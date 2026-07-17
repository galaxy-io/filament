import { match } from "ts-pattern";

import { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";

import { PipelineGroup, PipelineHealth } from "@/pages/pipelines/types";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

export const toPipelineGroups = (items: Pipeline[]): Record<PipelineGroup, Pipeline[]> => {
  const groups: Record<PipelineGroup, Pipeline[]> = {
    [PipelineGroup.ACTIVE]: [],
    [PipelineGroup.NEEDS_ATTENTION]: [],
    [PipelineGroup.PAUSED]: [],
  };
  for (const item of items) {
    groups[getPipelineGroup(item)].push(item);
  }
  return groups;
};

export const getPipelineGroup = (_pipeline: Pipeline): PipelineGroup => {
  // TODO: Implement when health is available on Pipeline
  return PipelineGroup.ACTIVE;
};

export const getHealthBeaconVariant = (health: PipelineHealth): BeaconVariant => {
  return match(health)
    .with(PipelineHealth.HEALTHY, () => BeaconVariant.SUCCESS)
    .with(PipelineHealth.DEGRADED, () => BeaconVariant.WARNING)
    .with(PipelineHealth.FAILING, () => BeaconVariant.ERROR)
    .with(PipelineHealth.PAUSED, () => BeaconVariant.DISABLED)
    .exhaustive();
};

export const formatCount = (value: bigint): string => {
  return Number(value).toLocaleString();
};

const BYTE_UNITS = ["B", "KB", "MB", "GB", "TB"];

export const formatBytes = (value: bigint): string => {
  let scaled = Number(value);
  let unitIndex = 0;
  while (scaled >= 1024 && unitIndex < BYTE_UNITS.length - 1) {
    scaled /= 1024;
    unitIndex += 1;
  }
  return `${unitIndex === 0 ? scaled : scaled.toFixed(1)} ${BYTE_UNITS[unitIndex]}`;
};
