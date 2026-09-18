import { type UseQueryOptions, useQuery } from "@connectrpc/connect-query";

import { IngestionService } from "@/gen/ingestion/v1/service_pb";
import type {
  ListTransformFunctionsRequest,
  ListTransformFunctionsResponse,
  ValidateTransformRequest,
  ValidateTransformResponse,
} from "@/gen/ingestion/v1/transformations_pb";

export const useListTransformFunctionsQuery = ({
  input,
  options = {},
}: {
  input: ListTransformFunctionsRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.listTransformFunctions.output,
    ListTransformFunctionsResponse
  >;
}) => {
  return useQuery<
    typeof IngestionService.method.listTransformFunctions.input,
    typeof IngestionService.method.listTransformFunctions.output
  >(IngestionService.method.listTransformFunctions, input, {
    staleTime: Number.POSITIVE_INFINITY,
    ...options,
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
  return useQuery<
    typeof IngestionService.method.validateTransform.input,
    typeof IngestionService.method.validateTransform.output
  >(IngestionService.method.validateTransform, input, { retry: false, ...options });
};
