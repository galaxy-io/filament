import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { useTransport } from "@connectrpc/connect-query";
import { useQueries } from "@tanstack/react-query";

import {
  GetPipelineVersionRequestSchema,
  type Pipeline,
  type PipelineNode,
} from "@/gen/ingestion/v1/pipelines_pb";

import { createGetPipelineVersionQueryOptions } from "@/api/queries/pipeline_versions";
import { useListPipelinesQuery } from "@/api/queries/pipelines";

export const usePipelineConnectionMap = () => {
  const transport = useTransport();
  const { data } = useListPipelinesQuery();
  const pipelines = useMemo(() => data?.pipelines ?? [], [data?.pipelines]);

  const versionResults = useQueries({
    queries: pipelines.map((pipeline) => ({
      ...createGetPipelineVersionQueryOptions({
        input: create(GetPipelineVersionRequestSchema, {
          pipelineId: pipeline.id,
        }),
        transport,
      }),
      retry: false,
    })),
  });

  const connectionIdsByPipelineId = useMemo(() => {
    const map = new Map<Pipeline["id"], Set<PipelineNode["connectionId"]>>();
    pipelines.forEach((pipeline, index) => {
      map.set(
        pipeline.id,
        new Set(
          (versionResults[index]?.data?.version?.graph?.nodes ?? []).map(
            (node) => node.connectionId,
          ),
        ),
      );
    });
    return map;
  }, [pipelines, versionResults]);

  return { pipelines, connectionIdsByPipelineId };
};
