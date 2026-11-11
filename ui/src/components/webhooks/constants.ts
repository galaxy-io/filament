import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";

import { WebhookEndpointStatus } from "@/components/webhooks/types";

export const WEBHOOK_CONNECTOR_NAME = "webhook";

export const WEBHOOK_DOCS_PATH = "/pages/connectors/sources/webhook";

export const WEBHOOK_RESOURCE_PLACEHOLDER = "orders...";

export const WEBHOOK_RESOURCE_TOOLTIP =
  "Letters, digits, - and _. The name becomes the last part of the delivery URL.";

export const WEBHOOK_RESOURCE_NAME_PATTERN = /^[A-Za-z0-9_-]{1,64}$/;

export const WEBHOOK_ENDPOINT_STATUS_TO_LABEL_MAP: Record<WebhookEndpointStatus, string> = {
  [WebhookEndpointStatus.NO_KEY]: "No key",
  [WebhookEndpointStatus.NO_PIPELINE]: "No pipeline",
  [WebhookEndpointStatus.PIPELINE_DELETED]: "Pipeline deleted",
  [WebhookEndpointStatus.OFF]: "Off",
  [WebhookEndpointStatus.NOT_RUNNING]: "Not running",
  [WebhookEndpointStatus.WAITING]: "Waiting",
  [WebhookEndpointStatus.LIVE]: "Live",
};

export const WEBHOOK_ENDPOINT_STATUS_TO_HUE_MAP: Record<
  WebhookEndpointStatus,
  RoleColor | undefined
> = {
  [WebhookEndpointStatus.NO_KEY]: undefined,
  [WebhookEndpointStatus.NO_PIPELINE]: undefined,
  [WebhookEndpointStatus.PIPELINE_DELETED]: "yellow",
  [WebhookEndpointStatus.OFF]: undefined,
  [WebhookEndpointStatus.NOT_RUNNING]: "yellow",
  [WebhookEndpointStatus.WAITING]: "orange",
  [WebhookEndpointStatus.LIVE]: "blue",
};

export const WEBHOOK_ENDPOINT_STATUS_TO_MESSAGE_MAP: Record<WebhookEndpointStatus, string> = {
  [WebhookEndpointStatus.NO_KEY]: "No signing key. Generate one under Secrets.",
  [WebhookEndpointStatus.NO_PIPELINE]: "Create a pipeline from this source to receive payloads.",
  [WebhookEndpointStatus.PIPELINE_DELETED]: "Pipeline deleted. Senders retry until a new one runs.",
  [WebhookEndpointStatus.OFF]: "Add a resource to the pipeline to receive payloads.",
  [WebhookEndpointStatus.NOT_RUNNING]: "Pipeline not running. Senders retry until it starts.",
  [WebhookEndpointStatus.WAITING]: "Payloads wait until the pipeline reads every resource.",
  [WebhookEndpointStatus.LIVE]: "These endpoints are live and can receive payloads.",
};
