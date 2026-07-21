import { useContext, useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { PipelineCanvasContext } from "@/pages/pipelines/canvas/PipelineCanvasProvider";
import type { PipelineNodeSourceTableInfo } from "@/pages/pipelines/canvas/types";
import {
  hasPipelineGraphChanges,
  mapCanvasStateToVersionRequest,
} from "@/pages/pipelines/canvas/utils";

import { useDiscoverResourcesQuery, useListConnectionsQuery } from "@/api/queries/connectors";
import { useCreatePipelineVersionMutation } from "@/api/queries/pipelines";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import type { Pipeline, PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";
import { DiscoverResourcesRequestSchema } from "@/gen/ingestion/v1/providers_pb";

export function usePipelineCanvas() {
  const context = useContext(PipelineCanvasContext);
  if (!context) {
    throw new Error("usePipelineCanvas must be used within PipelineCanvasProvider");
  }
  return context;
}

export const usePipelineCanvasSave = (
  pipeline: Pipeline,
  currentVersion: PipelineVersion | undefined,
) => {
  const { state } = usePipelineCanvas();
  const { showToast } = useToast();

  const { mutate: createPipelineVersion, isPending: isSaving } = useCreatePipelineVersionMutation();

  const hasChanges = useMemo(
    () => hasPipelineGraphChanges(state, currentVersion),
    [state, currentVersion],
  );

  // Saving appends an immutable graph version; the server advances the
  // pipeline's current version, so there is no optimistic-lock conflict to handle
  const save = () => {
    createPipelineVersion(mapCanvasStateToVersionRequest(state, pipeline.id, currentVersion), {
      onSuccess: () => {
        showToast({
          header: "Pipeline saved",
          subheader: `${pipeline.name} has been saved successfully.`,
          variant: ToastVariant.SUCCESS,
        });
      },
      onError: (error) => {
        showToast({
          header: "Save failed",
          subheader: error instanceof Error ? error.message : "Failed to save pipeline",
          variant: ToastVariant.ERROR,
        });
      },
    });
  };

  return { hasChanges, isSaving, save };
};

export const useSourceResources = (connectionId: string) => {
  const { data: connectionsData } = useListConnectionsQuery({
    input: create(ListConnectionsRequestSchema, { kind: ConnectorKind.SOURCE }),
  });

  const connection = connectionsData?.connections.find(({ id }) => id === connectionId);

  // Fail fast (no retry/pause) so the refresh button can always trigger a fresh fetch
  const { data, error, isFetching, refetch } = useDiscoverResourcesQuery({
    input: create(DiscoverResourcesRequestSchema, {
      connectionId: connection?.id ?? "",
    }),
    options: { enabled: Boolean(connection), retry: false, networkMode: "always" },
  });

  const tables = useMemo<PipelineNodeSourceTableInfo[]>(
    () =>
      data?.resources.map((resource) => ({
        name: resource.name,
        rowCount: String(resource.estimatedRows),
        isConnected: false,
      })) ?? [],
    [data?.resources],
  );

  const refresh = () => {
    void refetch();
  };

  return { tables, error, isLoading: isFetching, refresh };
};
