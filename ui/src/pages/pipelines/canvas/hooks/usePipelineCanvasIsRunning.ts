import { create } from "@bufbuild/protobuf";
import { useParams } from "@tanstack/react-router";

import { PaginationRequestSchema } from "@/gen/ingestion/v1/pagination_pb";
import { ListRunsRequestSchema } from "@/gen/ingestion/v1/runs_pb";

import { ACTIVE_RUN_STATUSES } from "@/api/queries/constants";
import { useListRunsQuery } from "@/api/queries/runs";

export const usePipelineCanvasIsRunning = (): boolean => {
  const { id } = useParams({ from: "/_app/pipelines/$id" });
  const { data } = useListRunsQuery({
    input: create(ListRunsRequestSchema, {
      pipelineId: id,
      status: [...ACTIVE_RUN_STATUSES],
      pagination: create(PaginationRequestSchema, { pageSize: 1 }),
    }),
  });
  return (data?.runs.length ?? 0) > 0;
};
