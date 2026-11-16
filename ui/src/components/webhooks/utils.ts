import { create } from "@bufbuild/protobuf";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  type ConnectorSpec,
  type Resource,
  WebhookSecretIssuer,
  type WebhookSpec,
} from "@/gen/ingestion/v1/connectors_pb";
import type { PipelineGraph } from "@/gen/ingestion/v1/pipelines_pb";
import {
  PutWebhookEndpointRequestSchema,
  type WebhookEndpointConfig,
  WebhookEndpointConfigSchema,
} from "@/gen/ingestion/v1/webhooks_pb";

import { formatConnectorName } from "@/components/connections/utils";
import {
  RESOURCE_INGRESS_WEBHOOK,
  RESOURCE_METADATA_INGRESS,
  WEBHOOK_CONNECTOR,
  WEBHOOK_CONTINUOUS_ROUTE_SUFFIX,
  WEBHOOK_INGESTION_TO_IS_RECEIVING_MAP,
  WEBHOOK_RESOURCE_NAME_PATTERN,
  WEBHOOK_SETUP_DELIVERY_URL_PATTERN,
  WEBHOOK_SETUP_DELIVERY_URL_UNAVAILABLE,
} from "@/components/webhooks/constants";
import { WebhookEndpointStatus, WebhookIngestion } from "@/components/webhooks/types";

export const isWebhookReceiving = (
  spec: WebhookSpec | undefined,
  config: Connection["config"] | undefined,
): spec is WebhookSpec =>
  !!spec && (!spec.ingestionField || isWebhookIngestionReceiving(config?.[spec.ingestionField]));

const isWebhookIngestion = (value: unknown): value is WebhookIngestion =>
  Object.values<unknown>(WebhookIngestion).includes(value);

const isWebhookIngestionReceiving = (value: unknown) =>
  isWebhookIngestion(value) && WEBHOOK_INGESTION_TO_IS_RECEIVING_MAP[value];

export const isWebhookProviderSecret = (spec: WebhookSpec | undefined) =>
  spec?.secretIssuer === WebhookSecretIssuer.PROVIDER;

export const isWebhookIngressResource = (resource: Resource) =>
  resource.metadata[RESOURCE_METADATA_INGRESS] === RESOURCE_INGRESS_WEBHOOK;

export const isWebhookSplittingExecution = (spec: WebhookSpec | undefined): spec is WebhookSpec =>
  !!spec?.splitsExecution;

export const getBoundedResources = (resources: string[], spec: WebhookSpec | undefined) =>
  isWebhookSplittingExecution(spec)
    ? resources.filter((resource) => !spec.resources.includes(resource))
    : resources;

export const isWebhookResource = (spec: WebhookSpec | undefined, resource: string) =>
  !!spec && resource !== "" && (spec.dynamicResources || spec.resources.includes(resource));

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
  resource: string | undefined,
  deliveryUrl: WebhookEndpointConfig["deliveryUrl"] | undefined,
) => {
  const baseUrl = createWebhookDeliveryBaseUrl(deliveryUrl);
  if (!baseUrl) return undefined;
  return resource ? `${baseUrl}/${resource}` : baseUrl;
};

export const createWebhookSetupContent = (
  setup: WebhookSpec["setup"],
  deliveryUrl: WebhookEndpointConfig["deliveryUrl"] | undefined,
) =>
  setup.replace(WEBHOOK_SETUP_DELIVERY_URL_PATTERN, (_, resource: string | undefined) => {
    const url = createWebhookDeliveryUrl(resource, deliveryUrl);
    return url ? `\`${url}\`` : WEBHOOK_SETUP_DELIVERY_URL_UNAVAILABLE;
  });

export const createWebhookRouteKey = (
  sourceNodeId: string,
  sinkNodeId: string,
  spec: WebhookSpec | undefined,
) =>
  isWebhookSplittingExecution(spec)
    ? `route/${sourceNodeId}/${sinkNodeId}${WEBHOOK_CONTINUOUS_ROUTE_SUFFIX}`
    : `route/${sourceNodeId}/${sinkNodeId}`;

export const getWebhookRouteResources = (
  graph: PipelineGraph | undefined,
  routeKey: WebhookEndpointConfig["routeKey"],
  spec: WebhookSpec | undefined,
) =>
  [
    ...new Set(
      (graph?.edges ?? [])
        .filter(
          (edge) =>
            isWebhookResource(spec, edge.resource) &&
            createWebhookRouteKey(edge.fromNode, edge.toNode, spec) === routeKey,
        )
        .map((edge) => edge.resource),
    ),
  ].sort();

export const getWebhookSourceRouteKeys = (
  graph: PipelineGraph | undefined,
  sourceConnectionId: Connection["id"],
  spec: WebhookSpec | undefined,
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
          (edge) => sourceNodeIds.has(edge.fromNode) && isWebhookResource(spec, edge.resource),
        )
        .map((edge) => createWebhookRouteKey(edge.fromNode, edge.toNode, spec)),
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

export const isWebhookProviderConnector = (connector: ConnectorSpec) =>
  connector.name !== WEBHOOK_CONNECTOR;

export const compareWebhookConnectors = (left: ConnectorSpec, right: ConnectorSpec) =>
  Number(isWebhookProviderConnector(left)) - Number(isWebhookProviderConnector(right)) ||
  formatConnectorName(left).localeCompare(formatConnectorName(right));
