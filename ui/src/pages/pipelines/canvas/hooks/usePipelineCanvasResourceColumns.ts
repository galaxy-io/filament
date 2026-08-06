import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import { GetResourceColumnsRequestSchema } from "@/gen/ingestion/v1/providers_pb";

import { useGetResourceColumnsQuery } from "@/api/queries/connectors";

const PIPELINE_CANVAS_RESOURCE_COLUMNS_STALE_TIME = 5 * 60 * 1000;

export const usePipelineCanvasResourceColumns = ({
  connectionId,
  resource,
  isEnabled,
}: {
  connectionId: string;
  resource: string;
  isEnabled: boolean;
}) => {
  const { data, error, isFetching, refetch } = useGetResourceColumnsQuery({
    input: create(GetResourceColumnsRequestSchema, { connectionId, resources: [resource] }),
    options: {
      enabled: isEnabled && connectionId !== "" && resource !== "",
      retry: false,
      networkMode: "always",
      staleTime: PIPELINE_CANVAS_RESOURCE_COLUMNS_STALE_TIME,
      refetchOnWindowFocus: false,
    },
  });

  const columns = useMemo(
    () => data?.resources.find((entry) => entry.resource === resource)?.columns ?? [],
    [data?.resources, resource],
  );

  return { columns, error, isLoading: isFetching, refresh: () => void refetch() };
};
