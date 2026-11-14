import { useTransport } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import {
  createPutWebhookEndpointRequest,
  getWebhookRouteResources,
  getWebhookSourceRouteKeys,
  isSameWebhookResources,
  isWebhookConnector,
} from "@/components/webhooks/utils";

import { createGetPipelineInput, createGetPipelineQueryOptions } from "@/api/queries/pipelines";
import {
  createGetWebhookEndpointInput,
  createGetWebhookEndpointQueryOptions,
  usePutWebhookEndpointMutation,
} from "@/api/queries/webhooks";

import { getErrorMessage, ignoreNotFoundError } from "@/utils/errors";

interface WebhookEndpointSyncInput {
  sourceConnection: Connection | null | undefined;
  pipelineId: Pipeline["id"];
}

const pendingSyncs = new Map<string, Promise<void>>();

export const useWebhookEndpointSync = () => {
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const transport = useTransport();
  const { mutateAsync: putEndpoint } = usePutWebhookEndpointMutation();

  const sync = async ({ sourceConnection, pipelineId }: WebhookEndpointSyncInput) => {
    if (!sourceConnection || !isWebhookConnector(sourceConnection.connector)) return;

    const sourceConnectionId = sourceConnection.id;

    try {
      const pipeline = await queryClient
        .fetchQuery({
          ...createGetPipelineQueryOptions({
            input: createGetPipelineInput(pipelineId),
            transport,
          }),
          staleTime: 0,
        })
        .then((response) => response.pipeline, ignoreNotFoundError);
      if (pipeline?.deletedAt !== 0n) return;
      const currentVersion = pipeline.currentVersion;
      if (!currentVersion?.graph) return;
      const graph = currentVersion.graph;
      const expectedPipelineVersionId = currentVersion.id;
      const routeKeys = getWebhookSourceRouteKeys(graph, sourceConnectionId);

      const endpoint = await queryClient
        .fetchQuery({
          ...createGetWebhookEndpointQueryOptions({
            input: createGetWebhookEndpointInput(sourceConnectionId),
            transport,
          }),
          staleTime: 0,
        })
        .then((response) => response.endpoint, ignoreNotFoundError);
      const isBoundHere = endpoint?.pipelineId === pipelineId;

      if (routeKeys.length === 0) {
        if (endpoint && isBoundHere && (endpoint.enabled || endpoint.resources.length > 0)) {
          await putEndpoint(
            createPutWebhookEndpointRequest({
              sourceConnectionId,
              endpoint,
              patch: { resources: [], enabled: false },
              expectedPipelineVersionId,
            }),
          );
        }
        return;
      }

      if (!endpoint?.hasSigningSecret) {
        toast({
          variant: ToastVariant.WARNING,
          header: "Webhook endpoint not set up",
          description: "Generate a signing key under Secrets on the source.",
        });
        return;
      }

      if (endpoint.pipelineId && !isBoundHere) {
        const boundPipeline = await queryClient
          .fetchQuery({
            ...createGetPipelineQueryOptions({
              input: createGetPipelineInput(endpoint.pipelineId),
              transport,
            }),
            staleTime: 0,
          })
          .then((response) => response.pipeline, ignoreNotFoundError);
        if (boundPipeline && boundPipeline.deletedAt === 0n) {
          toast({
            variant: ToastVariant.WARNING,
            header: "Webhook endpoint unchanged",
            description: `It stays bound to ${boundPipeline.name}.`,
          });
          return;
        }
      }

      const routeKey =
        isBoundHere && routeKeys.includes(endpoint.routeKey)
          ? endpoint.routeKey
          : (routeKeys[0] ?? "");
      const resources = getWebhookRouteResources(graph, routeKey);
      if (
        isBoundHere &&
        endpoint.routeKey === routeKey &&
        endpoint.enabled &&
        isSameWebhookResources(endpoint.resources, resources)
      ) {
        return;
      }

      await putEndpoint(
        createPutWebhookEndpointRequest({
          sourceConnectionId,
          endpoint,
          patch: { pipelineId, routeKey, resources, enabled: true },
          expectedPipelineVersionId,
        }),
      );
    } catch (error) {
      toast({
        variant: ToastVariant.ERROR,
        header: "Webhook endpoint not updated",
        description: getErrorMessage(error, "Failed to update the webhook endpoint"),
      });
    }
  };

  return (input: WebhookEndpointSyncInput) => {
    const sourceId = input.sourceConnection?.id;
    if (!sourceId) return Promise.resolve();
    const previous = pendingSyncs.get(sourceId) ?? Promise.resolve();
    const current = previous.then(() => sync(input));
    pendingSyncs.set(sourceId, current);
    void current.finally(() => {
      if (pendingSyncs.get(sourceId) === current) pendingSyncs.delete(sourceId);
    });
    return current;
  };
};
