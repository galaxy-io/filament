import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

export const formatPipelineName = (name: Pipeline["name"]): string => {
  return name.replace(/->/g, "→");
};
