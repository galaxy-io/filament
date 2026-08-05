import { create } from "@bufbuild/protobuf";
import { createClient, type Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  type UseMutationOptions,
  type UseQueryOptions,
  useMutation,
  useQuery,
  useSuspenseQuery,
  useTransport,
} from "@connectrpc/connect-query";
import { experimental_streamedQuery, useQueries, useQueryClient } from "@tanstack/react-query";

import {
  type GetRunRequest,
  type GetRunResponse,
  type ListRunsRequest,
  type RunEvent,
  type RunInfo,
  RunStatus,
  type TailRunRequest,
  TailRunRequestSchema,
  type TailRunResponse,
} from "@/gen/ingestion/v1/runs_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

export const ACTIVE_RUN_STATUSES = new Set<RunStatus>([
  RunStatus.REQUESTED,
  RunStatus.RUNNING,
  RunStatus.PAUSED,
]);

const LIST_RUNS_REFETCH_INTERVAL = 3 * 1000;
const GET_RUN_REFETCH_INTERVAL = 2 * 1000;
const MAX_TAIL_EVENTS = 2000;

export const createListRunsQueryKey = (input?: ListRunsRequest, transport?: Transport) => {
  return createConnectQueryKey({
    schema: IngestionService.method.listRuns,
    input,
    transport,
    cardinality: "finite",
  });
};

const getListRunsRefetchInterval = (runs: RunInfo[] | undefined) => {
  return runs?.some((run) => ACTIVE_RUN_STATUSES.has(run.status))
    ? LIST_RUNS_REFETCH_INTERVAL
    : false;
};

export const useListRunsQuery = ({ input }: { input?: ListRunsRequest } = {}) => {
  return useQuery<
    typeof IngestionService.method.listRuns.input,
    typeof IngestionService.method.listRuns.output
  >(IngestionService.method.listRuns, input, {
    refetchInterval: (query) => {
      return getListRunsRefetchInterval(query.state.data?.runs);
    },
  });
};

export const useSuspenseListRunsQuery = ({ input }: { input?: ListRunsRequest } = {}) => {
  return useSuspenseQuery<
    typeof IngestionService.method.listRuns.input,
    typeof IngestionService.method.listRuns.output
  >(IngestionService.method.listRuns, input, {
    refetchInterval: (query) => {
      return getListRunsRefetchInterval(query.state.data?.runs);
    },
  });
};

const getGetRunRefetchInterval = (status: RunStatus | undefined) => {
  return status !== undefined && ACTIVE_RUN_STATUSES.has(status) ? GET_RUN_REFETCH_INTERVAL : false;
};

export const useGetRunQuery = ({
  input,
  options = {},
}: {
  input: GetRunRequest;
  options?: UseQueryOptions<typeof IngestionService.method.getRun.output, GetRunResponse>;
}) => {
  return useQuery<
    typeof IngestionService.method.getRun.input,
    typeof IngestionService.method.getRun.output
  >(IngestionService.method.getRun, input, {
    refetchInterval: (query) => {
      return getGetRunRefetchInterval(query.state.data?.snapshot?.run?.status);
    },
    ...options,
  });
};

export const createTailRunQueryKey = (input?: TailRunRequest) => {
  return [IngestionService.method.tailRun.parent.typeName, input?.runId] as const;
};

export const useTailRunsStream = (runIds: string[]) => {
  const transport = useTransport();
  const results = useQueries({
    queries: runIds.map((runId) => {
      const input = create(TailRunRequestSchema, { runId, replay: true });
      return {
        queryKey: createTailRunQueryKey(input),
        queryFn: experimental_streamedQuery({
          streamFn: ({ signal }: { signal: AbortSignal }) =>
            createClient(IngestionService, transport).tailRun(input, {
              signal,
            }),
          reducer: (events: RunEvent[], response: TailRunResponse) =>
            response.event ? [...events, response.event].slice(-MAX_TAIL_EVENTS) : events,
          initialValue: [] as RunEvent[],
        }),
        gcTime: 0,
      };
    }),
  });

  return {
    events: results.flatMap((result) => result.data ?? []),
    isStreaming: results.some((result) => result.isFetching),
  };
};

export const useRunPipelineMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.runPipeline.input,
    typeof IngestionService.method.runPipeline.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof IngestionService.method.runPipeline.input,
    typeof IngestionService.method.runPipeline.output
  >(IngestionService.method.runPipeline, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListRunsQueryKey(),
      });
      return options.onSettled?.(...args);
    },
  });
};
