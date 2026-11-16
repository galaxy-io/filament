import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import { DiscoverResourcesRequestSchema, type Resource } from "@/gen/ingestion/v1/connectors_pb";

import { isWebhookIngressResource } from "@/components/webhooks/utils";

import { usePipelineExecutionLane } from "@/pages/pipelines/hooks/usePipelineExecutionLane";

import { useDiscoverResourcesQuery } from "@/api/queries/connectors";
import { PROBE_QUERY_OPTIONS } from "@/api/queries/constants";

export const usePipelineCanvasSourceResources = (connectionId: Connection["id"]) => {
  const { data, error, isFetching, refetch } = useDiscoverResourcesQuery({
    input: create(DiscoverResourcesRequestSchema, { connectionId }),
    options: { ...PROBE_QUERY_OPTIONS, enabled: connectionId !== "" },
  });
  const { isSplit, isStreamLane } = usePipelineExecutionLane();
  const resources = useMemo(
    () =>
      (data?.resources ?? []).filter(
        (resource) => !isSplit || isWebhookIngressResource(resource) === isStreamLane,
      ),
    [data?.resources, isSplit, isStreamLane],
  );

  const names = useMemo<Resource["name"][]>(
    () => resources.map((resource) => resource.name),
    [resources],
  );

  const selectableNames = useMemo<Resource["name"][]>(
    () => resources.filter((resource) => resource.isSelectable).map((resource) => resource.name),
    [resources],
  );

  return {
    names,
    selectableNames,
    error,
    isLoading: isFetching,
    refresh: () => void refetch(),
  };
};
