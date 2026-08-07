import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { stripDeletedName } from "@/utils/format";

export const formatPipelineName = (pipeline: Pipeline, includeDeleted = false): string => {
  const name = includeDeleted ? pipeline.name : stripDeletedName(pipeline.name);
  if (name) {
    return name.replace(/->/g, "→");
  }
  return pipeline.id;
};
