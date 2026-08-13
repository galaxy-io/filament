import { type UseMutationOptions, useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import { IngestionService } from "@/gen/ingestion/v1/service_pb";

import { createGetPipelineQueryKey } from "@/api/queries/pipelines";
import { createListRunsQueryKey } from "@/api/queries/runs";

export const useCreatePipelineScheduleMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.createPipelineSchedule.input,
    typeof IngestionService.method.createPipelineSchedule.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof IngestionService.method.createPipelineSchedule.input,
    typeof IngestionService.method.createPipelineSchedule.output
  >(IngestionService.method.createPipelineSchedule, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createGetPipelineQueryKey(),
      });
      // The server reconciles the schedule's pre-created SCHEDULED run rows
      // before responding, so every runs list is stale once this settles.
      void queryClient.invalidateQueries({
        queryKey: createListRunsQueryKey(),
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
  return useMutation<
    typeof IngestionService.method.updatePipelineSchedule.input,
    typeof IngestionService.method.updatePipelineSchedule.output
  >(IngestionService.method.updatePipelineSchedule, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createGetPipelineQueryKey(),
      });
      void queryClient.invalidateQueries({
        queryKey: createListRunsQueryKey(),
      });
      return options.onSettled?.(...args);
    },
  });
};
