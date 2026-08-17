import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  createInfiniteQueryOptions,
  createQueryOptions,
  type UseMutationOptions,
  type UseQueryOptions,
  useMutation,
  useQuery,
  useSuspenseInfiniteQuery,
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

import { getNextPageParam, INITIAL_PAGE_PARAM, type InfiniteQueryInput } from "@/api/utils";

const getListPipelinesRefetchInterval = (_pipelines: Pipeline[] | undefined): false => false;

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

export const createListPipelinesInfiniteQueryOptions = ({
  input,
  transport,
}: {
  input?: InfiniteQueryInput<typeof IngestionService.method.listPipelines.input>;
  transport: Transport;
}) => {
  return createInfiniteQueryOptions(
    IngestionService.method.listPipelines,
    { ...input, pagination: INITIAL_PAGE_PARAM },
    { transport, pageParamKey: "pagination", getNextPageParam },
  );
};

export const useSuspenseListPipelinesInfiniteQuery = ({
  input,
}: {
  input?: InfiniteQueryInput<typeof IngestionService.method.listPipelines.input>;
} = {}) => {
  return useSuspenseInfiniteQuery<
    typeof IngestionService.method.listPipelines.input,
    typeof IngestionService.method.listPipelines.output,
    "pagination"
  >(
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
