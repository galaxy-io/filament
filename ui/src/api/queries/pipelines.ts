import {
  createConnectQueryKey,
  useMutation,
  useQuery,
  UseMutationOptions,
  UseQueryOptions,
} from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import {
  GetPipelineRequest,
  GetPipelineResponse,
  ListPipelinesRequest,
  ListPipelinesResponse,
} from "@/gen/ingestion/v1/pipelines_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

// ========== LIST PIPELINES ==========

export const createListPipelinesQueryKey = (input?: ListPipelinesRequest) => {
  return createConnectQueryKey({
    schema: IngestionService.method.listPipelines,
    input,
    cardinality: "finite",
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
  >(IngestionService.method.listPipelines, input, options);
};

// ========== GET PIPELINE ==========

export const createGetPipelineQueryKey = (input: GetPipelineRequest) => {
  return createConnectQueryKey({
    schema: IngestionService.method.getPipeline,
    input,
    cardinality: "finite",
  });
};

export const useGetPipelineQuery = ({
  input,
  options = {},
}: {
  input: GetPipelineRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.getPipeline.output,
    GetPipelineResponse
  >;
}) => {
  return useQuery<
    typeof IngestionService.method.getPipeline.input,
    typeof IngestionService.method.getPipeline.output
  >(IngestionService.method.getPipeline, input, options);
};

// ========== MUTATIONS ==========

const useInvalidatePipelines = () => {
  const queryClient = useQueryClient();
  return () =>
    queryClient.invalidateQueries({
      queryKey: createConnectQueryKey({
        schema: IngestionService.method.listPipelines,
        cardinality: "finite",
      }),
    });
};

export const useCreatePipelineMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.createPipeline.input,
    typeof IngestionService.method.createPipeline.output
  > = {},
) => {
  const invalidate = useInvalidatePipelines();
  return useMutation(IngestionService.method.createPipeline, {
    ...options,
    onSettled: (...args) => {
      void invalidate();
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
  const invalidate = useInvalidatePipelines();
  return useMutation(IngestionService.method.updatePipeline, {
    ...options,
    onSettled: (...args) => {
      void invalidate();
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
  const invalidate = useInvalidatePipelines();
  return useMutation(IngestionService.method.deletePipeline, {
    ...options,
    onSettled: (...args) => {
      void invalidate();
      return options.onSettled?.(...args);
    },
  });
};
