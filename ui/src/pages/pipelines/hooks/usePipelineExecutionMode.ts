import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import { usePipelineParams } from "@/module/hooks";

import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

export const usePipelineExecutionMode = () => {
  const { id } = usePipelineParams();
  const { data } = useSuspenseGetPipelineQuery({ input: createGetPipelineInput(id) });
  return data.pipeline?.executionMode === ExecutionMode.CONTINUOUS
    ? ExecutionMode.CONTINUOUS
    : ExecutionMode.BOUNDED;
};
