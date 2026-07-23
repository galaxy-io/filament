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

// ABC Diatype's "->" ligature doesn't survive Text's letter-spacing, so
// render the real arrow glyph instead
export const formatPipelineName = (name: string): string => {
  return name.replace(/->/g, "→");
};

export const formatCount = (value: bigint): string => {
  return Number(value).toLocaleString();
};

export const formatTimeAgo = (unixMillis: bigint): string => {
  const elapsedMs = Date.now() - Number(unixMillis);
  const minutes = Math.floor(elapsedMs / 60_000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(Number(unixMillis)).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
  });
};

export const formatTimestamp = (unixMillis: bigint): string => {
  if (!unixMillis) return "—";
  return new Date(Number(unixMillis)).toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
  });
};

export const formatDuration = (startMillis: bigint, endMillis: bigint): string => {
  if (!startMillis || !endMillis) return "—";
  const elapsedMs = Number(endMillis - startMillis);
  if (elapsedMs < 1_000) return `${elapsedMs}ms`;
  const seconds = elapsedMs / 1_000;
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${Math.round(seconds % 60)}s`;
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
