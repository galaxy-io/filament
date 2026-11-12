import { create } from "@bufbuild/protobuf";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { PipelineGraph } from "@/gen/ingestion/v1/pipelines_pb";
import {
  PutWebhookEndpointRequestSchema,
  type WebhookEndpointConfig,
  WebhookEndpointConfigSchema,
} from "@/gen/ingestion/v1/webhooks_pb";

import {
  WEBHOOK_CONNECTOR_NAME,
  WEBHOOK_RESOURCE_NAME_PATTERN,
} from "@/components/webhooks/constants";
import { WebhookEndpointStatus } from "@/components/webhooks/types";

export const isWebhookConnector = (connector: Connection["connector"] | undefined) =>
  connector === WEBHOOK_CONNECTOR_NAME;

export const getWebhookEndpointStatus = (
  endpoint: WebhookEndpointConfig | undefined,
  isPipelineDeleted: boolean,
) => {
  if (!endpoint?.hasSigningSecret) return WebhookEndpointStatus.NO_KEY;
  if (!endpoint.pipelineId) return WebhookEndpointStatus.NO_PIPELINE;
  if (isPipelineDeleted) {
    return endpoint.enabled
      ? WebhookEndpointStatus.PIPELINE_DELETED
      : WebhookEndpointStatus.NO_PIPELINE;
  }
  if (!endpoint.enabled) return WebhookEndpointStatus.OFF;
  if (!endpoint.accepting) return WebhookEndpointStatus.NOT_RUNNING;
  if (!endpoint.routeReady) return WebhookEndpointStatus.WAITING;
  return WebhookEndpointStatus.LIVE;
};

export const isWebhookPreviousKeyActive = (endpoint: WebhookEndpointConfig | undefined) =>
  !!endpoint && endpoint.previousKeyExpiresAt > BigInt(Date.now());

export const createWebhookDeliveryBaseUrl = (
  deliveryUrl: WebhookEndpointConfig["deliveryUrl"] | undefined,
) => deliveryUrl || undefined;

export const createWebhookDeliveryUrl = (
  resource: string,
  deliveryUrl: WebhookEndpointConfig["deliveryUrl"] | undefined,
) => {
  const baseUrl = createWebhookDeliveryBaseUrl(deliveryUrl);
  return baseUrl ? `${baseUrl}/${resource}` : undefined;
};

export const createWebhookRouteKey = (sourceNodeId: string, sinkNodeId: string) =>
  `route/${sourceNodeId}/${sinkNodeId}`;

export const getWebhookRouteResources = (
  graph: PipelineGraph | undefined,
  routeKey: WebhookEndpointConfig["routeKey"],
) =>
  [
    ...new Set(
      (graph?.edges ?? [])
        .filter(
          (edge) =>
            edge.resource !== "" && createWebhookRouteKey(edge.fromNode, edge.toNode) === routeKey,
        )
        .map((edge) => edge.resource),
    ),
  ].sort();

export const getWebhookSourceRouteKeys = (
  graph: PipelineGraph | undefined,
  sourceConnectionId: Connection["id"],
) => {
  const sourceNodeIds = new Set(
    (graph?.nodes ?? [])
      .filter(
        (node) => node.kind === ConnectorKind.SOURCE && node.connectionId === sourceConnectionId,
      )
      .map((node) => node.id),
  );
  return [
    ...new Set(
      (graph?.edges ?? [])
        .filter((edge) => sourceNodeIds.has(edge.fromNode) && edge.resource !== "")
        .map((edge) => createWebhookRouteKey(edge.fromNode, edge.toNode)),
    ),
  ];
};

export const isSameWebhookResources = (left: string[], right: string[]) =>
  [...left].sort().join("\n") === [...right].sort().join("\n");

export const getWebhookResourceError = (resource: string) =>
  WEBHOOK_RESOURCE_NAME_PATTERN.test(resource)
    ? undefined
    : "Use only letters, digits, - and _, up to 64 characters.";

export const createPutWebhookEndpointRequest = ({
  sourceConnectionId,
  endpoint,
  patch = {},
  mintSecret = false,
}: {
  sourceConnectionId: Connection["id"];
  endpoint: WebhookEndpointConfig | undefined;
  patch?: Partial<Pick<WebhookEndpointConfig, "pipelineId" | "routeKey" | "resources" | "enabled">>;
  mintSecret?: boolean;
}) =>
  create(PutWebhookEndpointRequestSchema, {
    endpoint: create(WebhookEndpointConfigSchema, {
      sourceConnectionId,
      pipelineId: endpoint?.pipelineId ?? "",
      routeKey: endpoint?.routeKey ?? "",
      resources: endpoint?.resources ?? [],
      enabled: endpoint?.enabled ?? false,
      successStatus: endpoint?.successStatus ?? 0,
      maxBodyBytes: endpoint?.maxBodyBytes ?? 0n,
      ...patch,
    }),
    expectedRevision: endpoint?.revision ?? 0n,
    mintSecret,
  });
