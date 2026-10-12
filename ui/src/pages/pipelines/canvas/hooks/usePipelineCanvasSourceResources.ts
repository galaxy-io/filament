import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import { DiscoverResourcesRequestSchema, type Resource } from "@/gen/ingestion/v1/connectors_pb";

import { useDiscoverResourcesQuery } from "@/api/queries/connectors";
import { PROBE_QUERY_OPTIONS } from "@/api/queries/constants";

export const usePipelineCanvasSourceResources = (connectionId: Connection["id"]) => {
  const { data, error, isFetching, refetch } = useDiscoverResourcesQuery({
    input: create(DiscoverResourcesRequestSchema, { connectionId }),
    options: { ...PROBE_QUERY_OPTIONS, enabled: connectionId !== "" },
  });

  const names = useMemo<Resource["name"][]>(
    () => data?.resources.map((resource) => resource.name) ?? [],
    [data?.resources],
  );

  const selectableNames = useMemo<Resource["name"][]>(
    () =>
      (data?.resources ?? [])
        .filter((resource) => resource.isSelectable)
        .map((resource) => resource.name),
    [data?.resources],
  );

  return {
    names,
    selectableNames,
    error,
    isLoading: isFetching,
    refresh: () => void refetch(),
  };
};
