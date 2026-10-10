import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  type UseMutationOptions,
  type UseQueryOptions,
  useMutation,
  useQuery,
  useSuspenseInfiniteQuery,
  useSuspenseQuery,
  useTransport,
} from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import type {
  GetPipelineRequest,
  GetPipelineResponse,
  ListPipelinesRequest,
  ListPipelinesResponse,
  Pipeline,
} from "@/gen/ingestion/v1/pipelines_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

import type { ListSearchParams } from "@/module/schemas";

import { ACTIVE_RUNS_REFETCH_INTERVAL } from "@/api/queries/constants";
import { createListRunsQueryKey } from "@/api/queries/runs";
import {
  createListSearchInput,
  getNextPageParam,
  INITIAL_PAGE_PARAM,
  type InfiniteQueryInput,
} from "@/api/utils";

import { isRunActive } from "@/utils/runs";

const getListPipelinesRefetchInterval = (pipelines: Pipeline[] | undefined) => {
  return pipelines?.some((pipeline) => pipeline.lastRun && isRunActive(pipeline.lastRun))
    ? ACTIVE_RUNS_REFETCH_INTERVAL
    : false;
};

export const createListPipelinesInput = (search: ListSearchParams) => ({
  includeLastRun: true,
  includeSchedule: true,
  ...createListSearchInput(search),
});

export const createListPipelinesQueryKey = (
  input?: ListPipelinesRequest,
  transport?: Transport,
) => {
  return createConnectQueryKey({
    schema: IngestionService.method.listPipelines,
    input,
    transport,
    cardinality: undefined,
  });
};

export const useListPipelinesQuery = ({
  input,
  options = {},
}: {
  input?: ListPipelinesRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.listPipelines.output,
    ListPipelinesResponse
  >;
} = {}) => {
  return useQuery(IngestionService.method.listPipelines, input, {
    refetchInterval: (query) => {
      return getListPipelinesRefetchInterval(query.state.data?.pipelines);
    },
    ...options,
  });
};

export const useSuspenseListPipelinesQuery = ({ input }: { input?: ListPipelinesRequest } = {}) => {
  return useSuspenseQuery(IngestionService.method.listPipelines, input, {
    refetchInterval: (query) => {
      return getListPipelinesRefetchInterval(query.state.data?.pipelines);
    },
  });
};

export const useSuspenseListPipelinesInfiniteQuery = ({
  input,
}: {
  input?: InfiniteQueryInput<typeof IngestionService.method.listPipelines.input>;
} = {}) => {
  return useSuspenseInfiniteQuery(
    IngestionService.method.listPipelines,
    { ...input, pagination: INITIAL_PAGE_PARAM },
    {
      pageParamKey: "pagination",
      getNextPageParam,
      refetchInterval: (query) => {
        return getListPipelinesRefetchInterval(
          query.state.data?.pages.flatMap((page) => page.pipelines),
        );
      },
    },
  );
};

export const createGetPipelineQueryKey = (input?: GetPipelineRequest, transport?: Transport) => {
  return createConnectQueryKey({
    schema: IngestionService.method.getPipeline,
    input,
    transport,
    cardinality: "finite",
  });
};

export const useGetPipelineQuery = ({
  input,
  options = {},
}: {
  input: GetPipelineRequest;
  options?: UseQueryOptions<typeof IngestionService.method.getPipeline.output, GetPipelineResponse>;
}) => {
  return useQuery(IngestionService.method.getPipeline, input, options);
};

export const useSuspenseGetPipelineQuery = ({ input }: { input: GetPipelineRequest }) => {
  return useSuspenseQuery(IngestionService.method.getPipeline, input);
};

export const useCreatePipelineMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.createPipeline.input,
    typeof IngestionService.method.createPipeline.output
  > = {},
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();
  return useMutation(IngestionService.method.createPipeline, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListPipelinesQueryKey(undefined, transport),
      });
      return options.onSettled?.(...args);
    },
  });
};

export const useUpdatePipelineMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.updatePipeline.input,
    typeof IngestionService.method.updatePipeline.output
  > = {},
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();
  return useMutation(IngestionService.method.updatePipeline, {
    ...options,
    onSettled: (...args) => {
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

export const useDeletePipelineMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.deletePipeline.input,
    typeof IngestionService.method.deletePipeline.output
  > = {},
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();
  return useMutation(IngestionService.method.deletePipeline, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListPipelinesQueryKey(undefined, transport),
      });
      void queryClient.invalidateQueries({
        queryKey: createGetPipelineQueryKey(undefined, transport),
      });
      void queryClient.invalidateQueries({
        queryKey: createListRunsQueryKey(undefined, transport),
      });
      return options.onSettled?.(...args);
    },
  });
};
