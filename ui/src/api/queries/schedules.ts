import { type UseMutationOptions, useMutation, useTransport } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import { IngestionService } from "@/gen/ingestion/v1/service_pb";

import { createGetPipelineQueryKey, createListPipelinesQueryKey } from "@/api/queries/pipelines";
import { createListRunsQueryKey } from "@/api/queries/runs";

export const useCreatePipelineScheduleMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.createPipelineSchedule.input,
    typeof IngestionService.method.createPipelineSchedule.output
  > = {},
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();
  return useMutation(IngestionService.method.createPipelineSchedule, {
    ...options,
    onSettled: (...args) => {
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

export const useUpdatePipelineScheduleMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.updatePipelineSchedule.input,
    typeof IngestionService.method.updatePipelineSchedule.output
  > = {},
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();
  return useMutation(IngestionService.method.updatePipelineSchedule, {
    ...options,
    onSettled: (...args) => {
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
