import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { CancelledError } from "@tanstack/react-query";
import { createFileRoute, notFound } from "@tanstack/react-router";
import z from "zod";

import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineHistoryPage from "@/pages/pipelines/PipelineHistoryPage";

import { createGetPipelineQueryOptions } from "@/api/queries/pipelines";
import { createListRunsInfiniteQueryOptions } from "@/api/queries/runs";
import { queryClient } from "@/api/queryClient";
import { transport } from "@/api/transport";

const searchParams = z.object({
  runId: z.array(z.string()).optional().catch(undefined),
});

export const Route = createFileRoute("/_app/pipelines/$id/history")({
  validateSearch: searchParams,
  loader: async ({ params }) => {
    try {
      await Promise.all([
        queryClient.ensureQueryData(
          createGetPipelineQueryOptions({
            input: create(GetPipelineRequestSchema, { id: params.id, includeVersions: true }),
            transport,
          }),
        ),
        queryClient.ensureInfiniteQueryData(
          createListRunsInfiniteQueryOptions({ input: { pipelineId: params.id }, transport }),
        ),
      ]);
    } catch (error) {
      if (error instanceof CancelledError) {
        return;
      }
      if (error instanceof ConnectError && error.code === Code.NotFound) {
        throw notFound();
      }
      throw error;
    }
  },
  component: PipelineHistoryPage,
});
