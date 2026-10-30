import { ExecutionMode, ReplicationMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { PipelineRunResourceMode } from "@/pages/pipelines/components/run/types";

export const canPipelineRunSubset = (pipeline: Pipeline, source: Connection | undefined): boolean =>
  pipeline.executionMode !== ExecutionMode.CONTINUOUS &&
  !!source &&
  source.replication !== ReplicationMode.CDC;

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
