import { create } from "@bufbuild/protobuf";
import { useParams } from "@tanstack/react-router";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

export const usePipelineExecutionMode = () => {
  const { id } = useParams({ from: "/_app/pipelines/$id" });
  const { data } = useSuspenseGetPipelineQuery({ input: create(GetPipelineRequestSchema, { id }) });
  return data.pipeline?.executionMode === ExecutionMode.CONTINUOUS
    ? ExecutionMode.CONTINUOUS
    : ExecutionMode.BOUNDED;
};
