import { type UseQueryOptions, useQuery, useSuspenseQuery } from "@connectrpc/connect-query";

import { IngestionService } from "@/gen/ingestion/v1/service_pb";
import type {
  ListTransformFunctionsRequest,
  ValidateTransformRequest,
  ValidateTransformResponse,
} from "@/gen/ingestion/v1/transformations_pb";

export const useSuspenseListTransformFunctionsQuery = ({
  input,
}: {
  input: ListTransformFunctionsRequest;
}) => {
  return useSuspenseQuery(IngestionService.method.listTransformFunctions, input, {
    staleTime: Number.POSITIVE_INFINITY,
  });
};

export const useValidateTransformQuery = ({
  input,
  options = {},
}: {
  input: ValidateTransformRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.validateTransform.output,
    ValidateTransformResponse
  >;
}) => {
  return useQuery(IngestionService.method.validateTransform, input, { retry: false, ...options });
};
