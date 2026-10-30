import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import {
  ListPipelinesRequestSchema,
  type Pipeline,
  type PipelineNode,
} from "@/gen/ingestion/v1/pipelines_pb";

import { CONNECTION_PIPELINES_PAGE_SIZE } from "@/pages/connections/constants";

import { useListPipelinesQuery } from "@/api/queries/pipelines";

const CONNECTION_PIPELINES_INPUT = create(ListPipelinesRequestSchema, {
  includeSchedule: true,
  pagination: { pageSize: CONNECTION_PIPELINES_PAGE_SIZE },
});

export const useConnectionPipelines = () => {
  const { data, isError, isLoading } = useListPipelinesQuery({
    input: CONNECTION_PIPELINES_INPUT,
  });
  const pipelines = useMemo(() => data?.pipelines ?? [], [data?.pipelines]);

  const connectionIdsByPipelineId = useMemo(
    () =>
      new Map<Pipeline["id"], Set<PipelineNode["connectionId"]>>(
        pipelines.map((pipeline) => [
          pipeline.id,
          new Set((pipeline.currentVersion?.graph?.nodes ?? []).map((node) => node.connectionId)),
        ]),
      ),
    [pipelines],
  );

  return { pipelines, connectionIdsByPipelineId, isError, isLoading };
};
