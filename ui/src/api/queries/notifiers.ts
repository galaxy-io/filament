import type { DescMessage, DescMethodUnary } from "@bufbuild/protobuf";
import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  type UseMutationOptions,
  useMutation,
  useSuspenseQuery,
  useTransport,
} from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import type { ListPipelineNotifiersRequest } from "@/gen/ingestion/v1/notifiers_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

const createListPipelineNotifiersQueryKey = (
  input?: ListPipelineNotifiersRequest,
  transport?: Transport,
) =>
  createConnectQueryKey({
    schema: IngestionService.method.listPipelineNotifiers,
    input,
    transport,
    cardinality: "finite",
  });

export const useSuspenseListPipelineNotifiersQuery = ({
  input,
}: {
  input: ListPipelineNotifiersRequest;
}) => useSuspenseQuery(IngestionService.method.listPipelineNotifiers, input);

const usePipelineNotifierMutation = <I extends DescMessage, O extends DescMessage>(
  method: DescMethodUnary<I, O>,
  options: UseMutationOptions<I, O>,
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();
  return useMutation(method, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListPipelineNotifiersQueryKey(undefined, transport),
      });
      return options.onSettled?.(...args);
    },
  });
};

export const useCreatePipelineNotifierMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.createPipelineNotifier.input,
    typeof IngestionService.method.createPipelineNotifier.output
  > = {},
) => usePipelineNotifierMutation(IngestionService.method.createPipelineNotifier, options);

export const useUpdatePipelineNotifierMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.updatePipelineNotifier.input,
    typeof IngestionService.method.updatePipelineNotifier.output
  > = {},
) => usePipelineNotifierMutation(IngestionService.method.updatePipelineNotifier, options);

export const useDeletePipelineNotifierMutation = (
  options: UseMutationOptions<
    typeof IngestionService.method.deletePipelineNotifier.input,
    typeof IngestionService.method.deletePipelineNotifier.output
  > = {},
) => usePipelineNotifierMutation(IngestionService.method.deletePipelineNotifier, options);
