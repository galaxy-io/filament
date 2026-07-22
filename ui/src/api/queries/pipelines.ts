import { create } from "@bufbuild/protobuf";
import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  type UseMutationOptions,
  type UseQueryOptions,
  useMutation,
  useQuery,
  useTransport,
} from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import type {
  GetPipelineRequest,
  GetPipelineResponse,
  GetPipelineVersionRequest,
  GetPipelineVersionResponse,
  ListPipelinesRequest,
  ListPipelinesResponse,
  ListPipelineVersionsRequest,
  ListPipelineVersionsResponse,
} from "@/gen/ingestion/v1/pipelines_pb";
import {
  GetPipelineRequestSchema,
  GetPipelineResponseSchema,
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

// transport must be passed for exact-match operations (setQueryData): useQuery
// includes the context transport in its key, and unlike invalidateQueries
// (partial matching), setQueryData only writes to an exactly matching key
export const createGetPipelineQueryKey = (input: GetPipelineRequest, transport?: Transport) => {
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
  return useQuery<
    typeof IngestionService.method.getPipeline.input,
    typeof IngestionService.method.getPipeline.output
  >(IngestionService.method.getPipeline, input, options);
};

// ========== GET PIPELINE VERSION ==========

// version 0 resolves to the pipeline's current version on the server
export const createGetPipelineVersionQueryKey = (
  input?: GetPipelineVersionRequest,
  transport?: Transport,
) => {
  return createConnectQueryKey({
    schema: IngestionService.method.getPipelineVersion,
    input,
    transport,
    cardinality: "finite",
  });
};

export const useGetPipelineVersionQuery = ({
  input,
  options = {},
}: {
  input: GetPipelineVersionRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.getPipelineVersion.output,
    GetPipelineVersionResponse
  >;
}) => {
  return useQuery<
    typeof IngestionService.method.getPipelineVersion.input,
    typeof IngestionService.method.getPipelineVersion.output
  >(IngestionService.method.getPipelineVersion, input, options);
};

// ========== LIST PIPELINE VERSIONS ==========

export const useListPipelineVersionsQuery = ({
  input,
  options = {},
}: {
  input: ListPipelineVersionsRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.listPipelineVersions.output,
    ListPipelineVersionsResponse
  >;
}) => {
  return useQuery<
    typeof IngestionService.method.listPipelineVersions.input,
    typeof IngestionService.method.listPipelineVersions.output
  >(IngestionService.method.listPipelineVersions, input, options);
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

// Graph saves append an immutable version; the pipeline's current_version_id
// moves forward server-side, so both the pipeline and its version caches go stale
export const useCreatePipelineVersionMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.createPipelineVersion.input,
    typeof IngestionService.method.createPipelineVersion.output
  > = {},
) => {
  const queryClient = useQueryClient();
  const invalidate = useInvalidatePipelines();
  return useMutation(IngestionService.method.createPipelineVersion, {
    ...options,
    onSettled: (...args) => {
      void invalidate();
      void queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: IngestionService.method.getPipeline,
          cardinality: "finite",
        }),
      });
      void queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: IngestionService.method.getPipelineVersion,
          cardinality: "finite",
        }),
      });
      void queryClient.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: IngestionService.method.listPipelineVersions,
          cardinality: "finite",
        }),
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
  const invalidate = useInvalidatePipelines();
  return useMutation(IngestionService.method.updatePipeline, {
    ...options,
    onSuccess: (data, variables, onMutateResult, context) => {
      // Seed the item cache immediately so consumers see the bumped version without a refetch
      if (data.pipeline?.id) {
        queryClient.setQueryData(
          createGetPipelineQueryKey(
            create(GetPipelineRequestSchema, { id: data.pipeline.id }),
            transport,
          ),
          create(GetPipelineResponseSchema, { pipeline: data.pipeline }),
        );
      }
      return options.onSuccess?.(data, variables, onMutateResult, context);
    },
    onSettled: (data, error, ...rest) => {
      void invalidate();
      // Refetch the item on failure so a version conflict re-syncs the caller
      // with the server's current version before the next attempt
      if (error) {
        void queryClient.invalidateQueries({
          queryKey: createConnectQueryKey({
            schema: IngestionService.method.getPipeline,
            cardinality: "finite",
          }),
        });
      }
      return options.onSettled?.(data, error, ...rest);
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
