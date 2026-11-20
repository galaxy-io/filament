import { ExecutionMode, ReplicationMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import {
  type PipelineRunModalSeed,
  type PipelineRunModalState,
  PipelineRunResourceMode,
} from "@/pages/pipelines/components/run/types";

export const canPipelineRunSubset = (pipeline: Pipeline, source: Connection | undefined): boolean =>
  pipeline.executionMode !== ExecutionMode.CONTINUOUS &&
  !!source &&
  source.replication !== ReplicationMode.CDC;

export const canPipelineRunResync = (pipeline: Pipeline, source: Connection | undefined): boolean =>
  pipeline.executionMode !== ExecutionMode.CONTINUOUS ||
  source?.replication === ReplicationMode.CDC;

export const setPipelineRunResourceMode = (
  resources: string[],
  resyncResources: string[],
  targets: string[],
  mode: PipelineRunResourceMode,
): string[] =>
  resources.filter((resource) =>
    targets.includes(resource)
      ? mode === PipelineRunResourceMode.RESYNC
      : resyncResources.includes(resource),
  );

export const createPipelineRunModalState = ({
  resources,
  workerConfiguration,
  seed,
}: {
  resources: string[];
  workerConfiguration: PipelineRunModalState["workerConfiguration"];
  seed: PipelineRunModalSeed;
}): PipelineRunModalState => {
  const seededResources = resources.filter((resource) => seed.resources?.includes(resource));
  const selectedResources = seededResources.length ? seededResources : resources;
  const resyncResources = selectedResources.filter((resource) =>
    seed.resyncResources?.includes(resource),
  );
  return {
    workerConfiguration,
    selectedResources,
    resyncResources,
    isResourcesPaneOpen: selectedResources.length < resources.length || resyncResources.length > 0,
  };
};
