import { match } from "ts-pattern";

import { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";

import { ProviderKind } from "@/gen/ingestion/v1/common_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { PipelineGroup, PipelineHealth, PipelineListItem } from "@/pages/pipelines/types";

export const getPipelineGroup = (item: PipelineListItem): PipelineGroup => {
  return match(item.health)
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

/**
 * Maps a persisted Pipeline (proto) to a list item. Run metadata (last run,
 * volume, schedule) isn't part of the pipeline record yet, so those render as
 * "—" until the list view joins run state.
 */
export const toPipelineListItem = (pipeline: Pipeline): PipelineListItem => {
  const sources = pipeline.nodes.filter((node) => node.kind === ProviderKind.SOURCE);
  const sinks = pipeline.nodes.filter((node) => node.kind === ProviderKind.SINK);

  return {
    id: pipeline.id,
    name: pipeline.name || pipeline.id,
    health: PipelineHealth.HEALTHY,
    source: sources[0]?.provider ?? "unknown",
    sinks: sinks.map((node) => node.provider),
    lastRunLabel: "—",
    volumeLabel: "—",
    scheduleLabel: "—",
    isEnabled: true,
  };
};
