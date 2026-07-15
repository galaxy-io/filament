import { match } from "ts-pattern";

import { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";

import { PipelineGroup, PipelineHealth, type PipelineResource } from "@/pages/pipelines/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

export const toPipelineGroups = (
  items: PipelineResource[],
): Record<PipelineGroup, PipelineResource[]> => {
  const groups: Record<PipelineGroup, PipelineResource[]> = {
    [PipelineGroup.ACTIVE]: [],
    [PipelineGroup.NEEDS_ATTENTION]: [],
    [PipelineGroup.PAUSED]: [],
  };
  for (const item of items) {
    groups[getPipelineGroup(item)].push(item);
  }
  return groups;
};

export const getPipelineGroup = (pipeline: PipelineResource): PipelineGroup => {
  return match(pipeline.health)
    .with(PipelineHealth.HEALTHY, () => PipelineGroup.ACTIVE)
    .with(PipelineHealth.DEGRADED, PipelineHealth.FAILING, () => PipelineGroup.NEEDS_ATTENTION)
    .with(PipelineHealth.PAUSED, () => PipelineGroup.PAUSED)
    .exhaustive();
};

export const getHealthBeaconVariant = (health: PipelineHealth): BeaconVariant => {
  return match(health)
    .with(PipelineHealth.HEALTHY, () => BeaconVariant.SUCCESS)
    .with(PipelineHealth.DEGRADED, () => BeaconVariant.WARNING)
    .with(PipelineHealth.FAILING, () => BeaconVariant.ERROR)
    .with(PipelineHealth.PAUSED, () => BeaconVariant.DISABLED)
    .exhaustive();
};

export const toPipelineResource = (pipeline: Pipeline): PipelineResource => {
  const sources = pipeline.nodes.filter((node) => node.kind === ConnectorKind.SOURCE);
  const sinks = pipeline.nodes.filter((node) => node.kind === ConnectorKind.SINK);

  return {
    id: pipeline.id,
    name: pipeline.name || pipeline.id,
    health: PipelineHealth.HEALTHY,
    source: sources[0]?.connectionId ?? "",
    sinks: sinks.map((node) => node.connectionId),
    lastRunLabel: "—",
    volumeLabel: "—",
    scheduleLabel: "—",
    isEnabled: true,
  };
};
