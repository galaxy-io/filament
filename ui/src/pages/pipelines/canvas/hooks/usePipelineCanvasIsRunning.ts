import { create } from "@bufbuild/protobuf";

import { PaginationRequestSchema } from "@/gen/ingestion/v1/pagination_pb";
import { ListRunsRequestSchema } from "@/gen/ingestion/v1/runs_pb";

import { usePipelineParams } from "@/module/hooks";

import { useListRunsQuery } from "@/api/queries/runs";

import { ACTIVE_RUN_STATUSES } from "@/constants";

export const usePipelineCanvasIsRunning = (): boolean => {
  const { id } = usePipelineParams();
  const { data } = useListRunsQuery({
    input: create(ListRunsRequestSchema, {
      pipelineId: id,
      status: [...ACTIVE_RUN_STATUSES],
      pagination: create(PaginationRequestSchema, { pageSize: 1 }),
    }),
  });
  return (data?.runs.length ?? 0) > 0;
};
