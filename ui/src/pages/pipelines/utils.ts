import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

const DELETED_NAME_SUFFIX = /__deleted__\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/;

export const stripDeletedPipelineName = (name: string): string =>
  name.replace(DELETED_NAME_SUFFIX, "");

export const formatPipelineName = (pipeline: Pipeline, includeDeleted = false): string => {
  const name = includeDeleted ? pipeline.name : stripDeletedPipelineName(pipeline.name);
  if (name) {
    return name.replace(/->/g, "→");
  }
  return pipeline.id;
};
