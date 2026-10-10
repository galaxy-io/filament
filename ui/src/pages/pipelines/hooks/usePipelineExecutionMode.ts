import { create } from "@bufbuild/protobuf";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import { usePipelineParams } from "@/module/hooks";

import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

export const usePipelineExecutionMode = () => {
  const { id } = usePipelineParams();
  const { data } = useSuspenseGetPipelineQuery({ input: create(GetPipelineRequestSchema, { id }) });
  return data.pipeline?.executionMode === ExecutionMode.CONTINUOUS
    ? ExecutionMode.CONTINUOUS
    : ExecutionMode.BOUNDED;
};
