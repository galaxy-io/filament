import { create } from "@bufbuild/protobuf";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import { WebhookSecretIssuer, type WebhookSpec } from "@/gen/ingestion/v1/connectors_pb";
import type { PipelineGraph } from "@/gen/ingestion/v1/pipelines_pb";
import {
  PutWebhookEndpointRequestSchema,
  type WebhookEndpointConfig,
  WebhookEndpointConfigSchema,
} from "@/gen/ingestion/v1/webhooks_pb";

import { ConnectorTileBadge } from "@/components/connections/types";
import {
  WEBHOOK_CONTINUOUS_ROUTE_SUFFIX,
  WEBHOOK_RECEIVING_INGESTIONS,
  WEBHOOK_RESOURCE_NAME_PATTERN,
  WEBHOOK_SECRET_MIN_LENGTH,
  WEBHOOK_SETUP_DELIVERY_URL_PATTERN,
} from "@/components/webhooks/constants";
import {
  type WebhookDeliveryTarget,
  WebhookEndpointStatus,
  WebhookIngestion,
} from "@/components/webhooks/types";

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

export const createWebhookDeliveryUrl = (
  resource: string,
  deliveryUrl: WebhookEndpointConfig["deliveryUrl"] | undefined,
) => {
  if (!deliveryUrl) return undefined;
  return resource ? `${deliveryUrl}/${resource}` : deliveryUrl;
};

export const isWebhookResource = (webhook: WebhookSpec, resource: string) =>
  resource !== "" && (webhook.dynamicResources || webhook.resources.includes(resource));

export const createWebhookRouteKey = (
  sourceNodeId: string,
  sinkNodeId: string,
  webhook: WebhookSpec,
) =>
  `route/${sourceNodeId}/${sinkNodeId}${webhook.splitsExecution ? WEBHOOK_CONTINUOUS_ROUTE_SUFFIX : ""}`;

export const getWebhookRouteResources = (
  graph: PipelineGraph | undefined,
  routeKey: WebhookEndpointConfig["routeKey"],
  webhook: WebhookSpec,
) =>
  [
    ...new Set(
      (graph?.edges ?? [])
        .filter(
          (edge) =>
            isWebhookResource(webhook, edge.resource) &&
            createWebhookRouteKey(edge.fromNode, edge.toNode, webhook) === routeKey,
        )
        .map((edge) => edge.resource),
    ),
  ].sort();

export const getWebhookSourceRouteKeys = (
  graph: PipelineGraph | undefined,
  sourceConnectionId: Connection["id"],
  webhook: WebhookSpec,
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
        .filter(
          (edge) => sourceNodeIds.has(edge.fromNode) && isWebhookResource(webhook, edge.resource),
        )
        .map((edge) => createWebhookRouteKey(edge.fromNode, edge.toNode, webhook)),
    ),
  ];
};

export const isSameWebhookResources = (left: string[], right: string[]) =>
  [...left].sort().join("\n") === [...right].sort().join("\n");

export const getWebhookResourceError = (resource: string) =>
  WEBHOOK_RESOURCE_NAME_PATTERN.test(resource) ? undefined : "Use up to 64 letters, digits, - or _";

export const createPutWebhookEndpointRequest = ({
  sourceConnectionId,
  endpoint,
  patch = {},
  mintSecret = false,
  importedSecret = "",
  expectedPipelineVersionId,
}: {
  sourceConnectionId: Connection["id"];
  endpoint: WebhookEndpointConfig | undefined;
  patch?: Partial<Pick<WebhookEndpointConfig, "pipelineId" | "routeKey" | "resources" | "enabled">>;
  mintSecret?: boolean;
  importedSecret?: string;
  expectedPipelineVersionId?: string;
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
    importedSecret,
    expectedPipelineVersionId,
  });

export const getWebhookIngestion = (value: unknown): WebhookIngestion =>
  Object.values(WebhookIngestion).find((ingestion) => ingestion === value) ?? WebhookIngestion.API;

export const isWebhookReceiving = (
  webhook: WebhookSpec | undefined,
  config: Connection["config"] | undefined,
): webhook is WebhookSpec =>
  !!webhook &&
  (!webhook.ingestionField ||
    WEBHOOK_RECEIVING_INGESTIONS.includes(getWebhookIngestion(config?.[webhook.ingestionField])));

export const getWebhookDeliveryTargets = (
  webhook: WebhookSpec,
  endpointResources: string[] = [],
): WebhookDeliveryTarget[] => {
  if (webhook.routed) return [{ label: "All events", resource: "" }];
  const resources = webhook.dynamicResources ? endpointResources : webhook.resources;
  if (resources.length === 0) return [{ label: "Delivery URL", resource: "" }];
  return resources.map((resource) => ({ label: resource, resource }));
};

export const createWebhookSetupMarkdown = (setup: WebhookSpec["setup"], deliveryUrl: string) =>
  setup.replace(WEBHOOK_SETUP_DELIVERY_URL_PATTERN, (_, resource: string | undefined) =>
    deliveryUrl
      ? `\`${createWebhookDeliveryUrl(resource ?? "", deliveryUrl)}\``
      : "the delivery URL",
  );

export const getWebhookSecretError = (secret: string) =>
  secret !== "" && secret.length < WEBHOOK_SECRET_MIN_LENGTH
    ? `Use at least ${WEBHOOK_SECRET_MIN_LENGTH} characters`
    : undefined;

export const hasFixedWebhookResources = (webhook: WebhookSpec | undefined) =>
  !!webhook && !webhook.dynamicResources;

export const isWebhookProviderIssued = (webhook: WebhookSpec) =>
  webhook.secretIssuer === WebhookSecretIssuer.PROVIDER;

export const getWebhookConnectorTileBadge = (webhook: WebhookSpec | undefined) =>
  webhook?.ingestionField ? ConnectorTileBadge.WEBHOOKS : undefined;
