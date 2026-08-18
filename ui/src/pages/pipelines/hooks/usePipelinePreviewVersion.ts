import { create } from "@bufbuild/protobuf";
import { useParams, useSearch } from "@tanstack/react-router";

import { GetPipelineRequestSchema, type PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";

import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

export const usePipelinePreviewVersion = (): PipelineVersion | undefined => {
  const { id } = useParams({ from: "/pipelines/$id" });
  const { version: searchVersion } = useSearch({ from: "/pipelines/$id" });

  const { data } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id, includeVersions: true }),
  });

  return data.pipeline?.versions.find((item) => item.version === searchVersion);
};
