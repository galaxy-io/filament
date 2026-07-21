import { useContext, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { useQueryClient } from "@tanstack/react-query";

import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { PipelineCanvasContext } from "@/pages/pipelines/canvas/PipelineCanvasProvider";
import type { PipelineNodeSourceTableInfo } from "@/pages/pipelines/canvas/types";
import { hasPipelineGraphChanges, mapCanvasStateToPipeline } from "@/pages/pipelines/canvas/utils";

import { useDiscoverResourcesQuery, useListConnectionsQuery } from "@/api/queries/connectors";
import { createGetPipelineQueryKey, useUpdatePipelineMutation } from "@/api/queries/pipelines";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import {
  GetPipelineRequestSchema,
  type Pipeline,
  UpdatePipelineRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";
import { DiscoverResourcesRequestSchema } from "@/gen/ingestion/v1/providers_pb";

export function usePipelineCanvas() {
  const context = useContext(PipelineCanvasContext);
  if (!context) {
    throw new Error("usePipelineCanvas must be used within PipelineCanvasProvider");
  }
  return context;
}

export const usePipelineCanvasSave = (pipeline: Pipeline) => {
  const { state } = usePipelineCanvas();
  const { showToast } = useToast();
  const queryClient = useQueryClient();

  const { mutate: updatePipeline, isPending: isSaving } = useUpdatePipelineMutation();

  const hasChanges = useMemo(() => hasPipelineGraphChanges(state, pipeline), [state, pipeline]);

  const save = () => {
    updatePipeline(
      create(UpdatePipelineRequestSchema, {
        pipeline: mapCanvasStateToPipeline(state, pipeline),
      }),
      {
        onSuccess: () => {
          showToast({
            header: "Pipeline saved",
            subheader: `${pipeline.name} has been saved successfully.`,
            variant: ToastVariant.SUCCESS,
          });
        },
        onError: (error) => {
          const isVersionConflict = error instanceof ConnectError && error.code === Code.Aborted;

          if (isVersionConflict) {
            // Someone else saved since we loaded - refetch so the next save carries the latest version
            void queryClient.invalidateQueries({
              queryKey: createGetPipelineQueryKey(
                create(GetPipelineRequestSchema, { id: pipeline.id }),
              ),
            });
          }

          showToast({
            header: "Save failed",
            subheader: isVersionConflict
              ? "This pipeline was changed elsewhere. The latest version has been reloaded - please try again."
              : error instanceof Error
                ? error.message
                : "Failed to save pipeline",
            variant: ToastVariant.ERROR,
          });
        },
      },
    );
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
