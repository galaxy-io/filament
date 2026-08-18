import { type UseQueryOptions, useQuery } from "@connectrpc/connect-query";

import type {
  ValidatePipelineRequest,
  ValidatePipelineResponse,
} from "@/gen/ingestion/v1/capabilities_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

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
