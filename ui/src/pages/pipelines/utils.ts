import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

export const formatPipelineName = (pipeline: Pipeline): string => {
  if (pipeline.name) {
    return pipeline.name.replace(/->/g, "→");
  }
  return pipeline.id;
};
