import { type UseQueryOptions, useQuery } from "@connectrpc/connect-query";

import { IngestionService } from "@/gen/ingestion/v1/service_pb";
import type { GetVersionResponse } from "@/gen/ingestion/v1/version_pb";

export const useGetVersionQuery = ({
  options = {},
}: {
  options?: UseQueryOptions<typeof IngestionService.method.getVersion.output, GetVersionResponse>;
} = {}) => {
  return useQuery(
    IngestionService.method.getVersion,
    {},
    {
      staleTime: Number.POSITIVE_INFINITY,
      ...options,
    },
  );
};
