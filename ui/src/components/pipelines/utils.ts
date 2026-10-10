import { ConnectorKind, ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Pipeline, PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";

import { PIPELINE_UNTITLED_NAME } from "@/components/pipelines/constants";

const DELETED_NAME_SUFFIX = /__deleted__\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/;

export const stripDeletedName = (name: string) => name.replace(DELETED_NAME_SUFFIX, "");

export interface PipelineFlowEndpoints {
  sourceId?: Connection["id"];
  sinkIds: Connection["id"][];
}

export const mapVersionNodesToFlowEndpoints = (
  nodes: NonNullable<PipelineVersion["graph"]>["nodes"],
): PipelineFlowEndpoints => ({
  sourceId: nodes.find((node) => node.kind === ConnectorKind.SOURCE)?.connectionId,
  sinkIds: nodes
    .filter((node) => node.kind === ConnectorKind.SINK)
    .map((node) => node.connectionId),
});

export const formatPipelineName = (pipeline: Pipeline, includeDeleted = false): string => {
  const name = includeDeleted ? pipeline.name : stripDeletedName(pipeline.name);
  return name ? name.replace(/->/g, "→") : PIPELINE_UNTITLED_NAME;
};

export const isPipelineNameMatch = (typed: string, name: string): boolean =>
  typed.trim().replace(/->/g, "→") === name.trim();

export const getPipelineNextFireAt = (pipeline: Pipeline | undefined) => {
  const nextFireAt = pipeline?.schedule?.nextFireAt;
  if (pipeline?.executionMode === ExecutionMode.CONTINUOUS) return undefined;
  return pipeline?.schedule?.config?.isEnabled && nextFireAt ? nextFireAt : undefined;
};
