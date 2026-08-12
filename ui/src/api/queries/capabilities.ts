import type { Transport } from "@connectrpc/connect";
import { createConnectQueryKey, type UseQueryOptions, useQuery } from "@connectrpc/connect-query";

import type {
  GetConnectionCapabilitiesRequest,
  GetConnectionCapabilitiesResponse,
  ValidatePipelineRequest,
  ValidatePipelineResponse,
} from "@/gen/ingestion/v1/capabilities_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

export const createGetConnectionCapabilitiesQueryKey = (
  input?: GetConnectionCapabilitiesRequest,
  transport?: Transport,
) => {
  return createConnectQueryKey({
    schema: IngestionService.method.getConnectionCapabilities,
    input,
    transport,
    cardinality: "finite",
  });
};

export const useGetConnectionCapabilitiesQuery = ({
  input,
  options = {},
}: {
  input: GetConnectionCapabilitiesRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.getConnectionCapabilities.output,
    GetConnectionCapabilitiesResponse
  >;
}) => {
  return useQuery<
    typeof IngestionService.method.getConnectionCapabilities.input,
    typeof IngestionService.method.getConnectionCapabilities.output
  >(IngestionService.method.getConnectionCapabilities, input, { retry: false, ...options });
};

export const useValidatePipelineQuery = ({
  input,
  options = {},
}: {
  input: ValidatePipelineRequest;
  options?: UseQueryOptions<
    typeof IngestionService.method.validatePipeline.output,
    ValidatePipelineResponse
  >;
}) => {
  return useQuery<
    typeof IngestionService.method.validatePipeline.input,
    typeof IngestionService.method.validatePipeline.output
  >(IngestionService.method.validatePipeline, input, { retry: false, ...options });
};
