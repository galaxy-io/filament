import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  createQueryOptions,
  type UseMutationOptions,
  type UseQueryOptions,
  useMutation,
  useQuery,
  useSuspenseQuery,
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

import { ACTIVE_RUN_STATUSES } from "@/api/queries/constants";

const LIST_PIPELINES_REFETCH_INTERVAL = 3 * 1000;

const getListPipelinesRefetchInterval = (pipelines: Pipeline[] | undefined) => {
  return pipelines?.some((pipeline) => ACTIVE_RUN_STATUSES.has(pipeline.lastRunStatus))
    ? LIST_PIPELINES_REFETCH_INTERVAL
    : false;
};

export const createListPipelinesQueryKey = (
  input?: ListPipelinesRequest,
  transport?: Transport,
) => {
  return createConnectQueryKey({
    schema: IngestionService.method.listPipelines,
    input,
    transport,
    cardinality: "finite",
  });
};

export const createListPipelinesQueryOptions = ({
  input,
  transport,
}: {
  input?: ListPipelinesRequest;
  transport: Transport;
}) => {
  return createQueryOptions(IngestionService.method.listPipelines, input, {
    transport,
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
  return useQuery<
    typeof IngestionService.method.listPipelines.input,
    typeof IngestionService.method.listPipelines.output
  >(IngestionService.method.listPipelines, input, {
    refetchInterval: (query) => {
      return getListPipelinesRefetchInterval(query.state.data?.pipelines);
    },
    ...options,
  });
};

export const useSuspenseListPipelinesQuery = ({ input }: { input?: ListPipelinesRequest } = {}) => {
  return useSuspenseQuery<
    typeof IngestionService.method.listPipelines.input,
    typeof IngestionService.method.listPipelines.output
  >(IngestionService.method.listPipelines, input, {
    refetchInterval: (query) => {
      return getListPipelinesRefetchInterval(query.state.data?.pipelines);
    },
  });
};

export const createGetPipelineQueryKey = (input?: GetPipelineRequest, transport?: Transport) => {
  return createConnectQueryKey({
    schema: IngestionService.method.getPipeline,
    input,
    transport,
    cardinality: "finite",
  });
};

export const createGetPipelineQueryOptions = ({
  input,
  transport,
}: {
  input: GetPipelineRequest;
  transport: Transport;
}) => {
  return createQueryOptions(IngestionService.method.getPipeline, input, {
    transport,
  });
};

export const useGetPipelineQuery = ({
  input,
  options = {},
}: {
  input: GetPipelineRequest;
  options?: UseQueryOptions<typeof IngestionService.method.getPipeline.output, GetPipelineResponse>;
}) => {
  return useQuery<
    typeof IngestionService.method.getPipeline.input,
    typeof IngestionService.method.getPipeline.output
  >(IngestionService.method.getPipeline, input, options);
};

export const useSuspenseGetPipelineQuery = ({ input }: { input: GetPipelineRequest }) => {
  return useSuspenseQuery<
    typeof IngestionService.method.getPipeline.input,
    typeof IngestionService.method.getPipeline.output
  >(IngestionService.method.getPipeline, input);
};

export const useCreatePipelineMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.createPipeline.input,
    typeof IngestionService.method.createPipeline.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof IngestionService.method.createPipeline.input,
    typeof IngestionService.method.createPipeline.output
  >(IngestionService.method.createPipeline, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListPipelinesQueryKey(),
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
  return useMutation<
    typeof IngestionService.method.updatePipeline.input,
    typeof IngestionService.method.updatePipeline.output
  >(IngestionService.method.updatePipeline, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListPipelinesQueryKey(),
      });
      void queryClient.invalidateQueries({
        queryKey: createGetPipelineQueryKey(),
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
  return useMutation<
    typeof IngestionService.method.deletePipeline.input,
    typeof IngestionService.method.deletePipeline.output
  >(IngestionService.method.deletePipeline, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListPipelinesQueryKey(),
      });
      void queryClient.invalidateQueries({
        queryKey: createGetPipelineQueryKey(),
      });
      return options.onSettled?.(...args);
    },
  });
};
