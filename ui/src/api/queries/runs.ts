import { create } from "@bufbuild/protobuf";
import { createClient, type Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  type UseMutationOptions,
  type UseQueryOptions,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useSuspenseInfiniteQuery,
  useSuspenseQuery,
  useTransport,
} from "@connectrpc/connect-query";
import {
  experimental_streamedQuery,
  type InfiniteData,
  useQueries,
  useQueryClient,
} from "@tanstack/react-query";

import {
  type GetRunRequest,
  type GetRunResponse,
  type ListRunsRequest,
  ListRunsRequestSchema,
  type ListRunsResponse,
  type RunEvent,
  type RunInfo,
  RunStatus,
  type TailRunRequest,
  TailRunRequestSchema,
  type TailRunResponse,
} from "@/gen/ingestion/v1/runs_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

import {
  ACTIVE_PIPELINE_RUNS_PAGE_SIZE,
  ACTIVE_RUNS_REFETCH_INTERVAL,
} from "@/api/queries/constants";
import {
  createGetPipelineInput,
  createGetPipelineQueryKey,
  createListPipelinesQueryKey,
  useGetPipelineQuery,
} from "@/api/queries/pipelines";
import {
  batchIterable,
  getNextPageParam,
  INITIAL_PAGE_PARAM,
  type InfiniteQueryInput,
  type UseInfiniteQueryOptions,
  type UseSuspenseQueryOptions,
} from "@/api/utils";

import { ACTIVE_RUN_STATUSES } from "@/constants";

import { isRunActive } from "@/utils/runs";

const GET_RUN_REFETCH_INTERVAL = 2 * 1000;
const MAX_TAIL_EVENTS = 2000;
const TAIL_FLUSH_INTERVAL = 150;

export const createListRunsQueryKey = (input?: ListRunsRequest, transport?: Transport) => {
  return createConnectQueryKey({
    schema: IngestionService.method.listRuns,
    input,
    transport,
    cardinality: undefined,
  });
};

const SCHEDULED_REFETCH_MAX_INTERVAL = 60 * 1000;
const IDLE_RUNS_REFETCH_INTERVAL = 30 * 1000;

const getScheduledRefetchInterval = (runs: RunInfo[], floor: number) => {
  const scheduled = runs.filter((run) => run.status === RunStatus.SCHEDULED);
  if (scheduled.length === 0) return false as const;
  const waits = scheduled.map((run) =>
    run.scheduledAt ? Number(run.scheduledAt) - Date.now() : SCHEDULED_REFETCH_MAX_INTERVAL,
  );
  return Math.min(Math.max(Math.min(...waits), floor), SCHEDULED_REFETCH_MAX_INTERVAL);
};

const getListRunsRefetchInterval = (runs: RunInfo[] | undefined) => {
  if (!runs) return false;
  if (runs.some((run) => isRunActive(run))) return ACTIVE_RUNS_REFETCH_INTERVAL;
  return getScheduledRefetchInterval(runs, ACTIVE_RUNS_REFETCH_INTERVAL);
};

const getActiveRunsRefetchInterval = (
  runs: RunInfo[] | undefined,
  nextFireAt: bigint | undefined,
) => {
  if (runs?.some((run) => isRunActive(run))) return ACTIVE_RUNS_REFETCH_INTERVAL;
  if (!nextFireAt) return IDLE_RUNS_REFETCH_INTERVAL;
  const wait = Number(nextFireAt) - Date.now();
  return Math.min(Math.max(wait, ACTIVE_RUNS_REFETCH_INTERVAL), IDLE_RUNS_REFETCH_INTERVAL);
};

export const useListRunsQuery = ({
  input,
  options = {},
}: {
  input?: ListRunsRequest;
  options?: UseQueryOptions<typeof IngestionService.method.listRuns.output, ListRunsResponse>;
} = {}) => {
  return useQuery(IngestionService.method.listRuns, input, {
    refetchInterval: (query) => {
      return getListRunsRefetchInterval(query.state.data?.runs);
    },
    ...options,
  });
};

export const useSuspenseListRunsQuery = ({
  input,
  options = {},
}: {
  input?: ListRunsRequest;
  options?: UseSuspenseQueryOptions<
    typeof IngestionService.method.listRuns.input,
    typeof IngestionService.method.listRuns.output
  >;
} = {}) => {
  return useSuspenseQuery(IngestionService.method.listRuns, input, {
    refetchInterval: (query) => {
      return getListRunsRefetchInterval(query.state.data?.runs);
    },
    ...options,
  });
};

const createListActivePipelineRunsInput = (pipelineId: RunInfo["pipelineId"]) =>
  create(ListRunsRequestSchema, {
    pipelineId,
    status: [...ACTIVE_RUN_STATUSES],
    pagination: { pageSize: ACTIVE_PIPELINE_RUNS_PAGE_SIZE },
  });

const usePipelineScheduledFireAt = (pipelineId: RunInfo["pipelineId"]) => {
  const { data } = useGetPipelineQuery({ input: createGetPipelineInput(pipelineId) });
  const schedule = data?.pipeline?.schedule;
  return schedule?.config?.isEnabled ? schedule.nextFireAt : undefined;
};

export const useListActivePipelineRunsQuery = (pipelineId: RunInfo["pipelineId"]) => {
  const nextFireAt = usePipelineScheduledFireAt(pipelineId);
  return useQuery(IngestionService.method.listRuns, createListActivePipelineRunsInput(pipelineId), {
    refetchInterval: (query) => getActiveRunsRefetchInterval(query.state.data?.runs, nextFireAt),
  });
};

export const useSuspenseListActivePipelineRunsQuery = (pipelineId: RunInfo["pipelineId"]) => {
  const nextFireAt = usePipelineScheduledFireAt(pipelineId);
  return useSuspenseQuery(
    IngestionService.method.listRuns,
    createListActivePipelineRunsInput(pipelineId),
    {
      refetchInterval: (query) => getActiveRunsRefetchInterval(query.state.data?.runs, nextFireAt),
    },
  );
};

const selectUniqueRunPages = (data: InfiniteData<ListRunsResponse>) => {
  const seen = new Set<RunInfo["id"]>();
  return {
    ...data,
    pages: data.pages.map((page) => ({
      ...page,
      runs: page.runs.filter((run) => {
        if (seen.has(run.id)) return false;
        seen.add(run.id);
        return true;
      }),
    })),
  };
};

export const useListRunsInfiniteQuery = ({
  input,
  options = {},
}: {
  input?: InfiniteQueryInput<typeof IngestionService.method.listRuns.input>;
  options?: UseInfiniteQueryOptions<
    typeof IngestionService.method.listRuns.input,
    typeof IngestionService.method.listRuns.output,
    "pagination"
  >;
} = {}) => {
  return useInfiniteQuery(
    IngestionService.method.listRuns,
    { ...input, pagination: INITIAL_PAGE_PARAM },
    {
      pageParamKey: "pagination",
      getNextPageParam,
      select: selectUniqueRunPages,
      refetchInterval: (query) => {
        return getListRunsRefetchInterval(query.state.data?.pages.flatMap((page) => page.runs));
      },
      ...options,
    },
  );
};

export const useSuspenseListRunsInfiniteQuery = ({
  input,
}: {
  input?: InfiniteQueryInput<typeof IngestionService.method.listRuns.input>;
} = {}) => {
  return useSuspenseInfiniteQuery(
    IngestionService.method.listRuns,
    { ...input, pagination: INITIAL_PAGE_PARAM },
    {
      pageParamKey: "pagination",
      getNextPageParam,
      select: selectUniqueRunPages,
      refetchInterval: (query) => {
        return getListRunsRefetchInterval(query.state.data?.pages.flatMap((page) => page.runs));
      },
    },
  );
};

const getGetRunRefetchInterval = (run: RunInfo | undefined) => {
  if (!run) return false;
  if (isRunActive(run)) return GET_RUN_REFETCH_INTERVAL;
  return getScheduledRefetchInterval([run], GET_RUN_REFETCH_INTERVAL);
};

export const useGetRunQuery = ({
  input,
  options = {},
}: {
  input: GetRunRequest;
  options?: UseQueryOptions<typeof IngestionService.method.getRun.output, GetRunResponse>;
}) => {
  return useQuery(IngestionService.method.getRun, input, {
    refetchInterval: (query) => {
      return getGetRunRefetchInterval(query.state.data?.snapshot?.run);
    },
    ...options,
  });
};

export const createTailRunQueryKey = (input?: TailRunRequest) => {
  return [IngestionService.method.tailRun.parent.typeName, input?.runId] as const;
};

export const useTailRunsStream = (runIds: RunInfo["id"][]) => {
  const transport = useTransport();
  const results = useQueries({
    queries: runIds.map((runId) => {
      const input = create(TailRunRequestSchema, { runId, shouldReplay: true });
      return {
        queryKey: createTailRunQueryKey(input),
        queryFn: experimental_streamedQuery({
          streamFn: ({ signal }: { signal: AbortSignal }) =>
            batchIterable(
              createClient(IngestionService, transport).tailRun(input, {
                signal,
              }),
              TAIL_FLUSH_INTERVAL,
            ),
          reducer: (events: RunEvent[], responses: TailRunResponse[]) => {
            const incoming = responses.flatMap((response) => response.event ?? []);
            return incoming.length > 0 ? [...events, ...incoming].slice(-MAX_TAIL_EVENTS) : events;
          },
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
  const transport = useTransport();
  return useMutation(IngestionService.method.runPipeline, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListRunsQueryKey(undefined, transport),
      });
      void queryClient.invalidateQueries({
        queryKey: createListPipelinesQueryKey(undefined, transport),
      });
      void queryClient.invalidateQueries({
        queryKey: createGetPipelineQueryKey(undefined, transport),
      });
      return options.onSettled?.(...args);
    },
  });
};

export const useSignalRunMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.signalRun.input,
    typeof IngestionService.method.signalRun.output
  > = {},
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();
  return useMutation(IngestionService.method.signalRun, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: IngestionService.method.getRun,
          cardinality: undefined,
        }),
      });
      void queryClient.invalidateQueries({
        queryKey: createGetPipelineQueryKey(undefined, transport),
      });
      void queryClient.invalidateQueries({
        queryKey: createListPipelinesQueryKey(undefined, transport),
      });
      void queryClient.invalidateQueries({
        queryKey: createListRunsQueryKey(undefined, transport),
      });
      return options.onSettled?.(...args);
    },
  });
};
