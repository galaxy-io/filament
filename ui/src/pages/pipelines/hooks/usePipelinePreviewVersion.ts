import type { PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";

import { usePipelineParams, usePipelineSearch } from "@/module/hooks";

import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

export const usePipelinePreviewVersion = (): PipelineVersion | undefined => {
  const { id } = usePipelineParams();
  const { version: searchVersion } = usePipelineSearch();

  const { data } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(id),
  });

  return data.pipeline?.versions.find((item) => item.version === searchVersion);
};
