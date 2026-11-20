import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";

import { WebhookEndpointStatus, WebhookIngestion } from "@/components/webhooks/types";

export const WEBHOOK_DOCS_PATH = "/pages/connectors/sources/webhook";

export const WEBHOOK_RESOURCE_PLACEHOLDER = "orders...";

export const WEBHOOK_RESOURCE_TOOLTIP =
  "Letters, digits, - and _. The name becomes the last part of the delivery URL.";

export const WEBHOOK_RESOURCE_NAME_PATTERN = /^[A-Za-z0-9_-]{1,64}$/;

export const WEBHOOK_SETUP_DELIVERY_URL_PATTERN = /\{\{\s*delivery_url(?::([^}\s]+))?\s*\}\}/g;

export const WEBHOOK_RECEIVING_INGESTIONS: WebhookIngestion[] = [
  WebhookIngestion.WEBHOOKS,
  WebhookIngestion.BOTH,
];

export const WEBHOOK_CONTINUOUS_ROUTE_SUFFIX = "/continuous";

export const WEBHOOK_SECRET_MIN_LENGTH = 16;

export const WEBHOOK_DYNAMIC_DELIVERY_URL_TOOLTIP =
  "Send each delivery to this URL followed by a resource name, such as /orders. Resources are named in the pipeline that reads this source.";

export const WEBHOOK_DELIVERY_URL_UNAVAILABLE_MESSAGE =
  "Set WEBHOOK_PUBLIC_BASE_URL on the server to show delivery URLs.";

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
  [WebhookEndpointStatus.NO_KEY]: "Create a signing key under Secrets.",
  [WebhookEndpointStatus.NO_PIPELINE]: "Create a pipeline from this source to receive deliveries.",
  [WebhookEndpointStatus.PIPELINE_DELETED]: "Senders retry until a new pipeline reads this source.",
  [WebhookEndpointStatus.OFF]: "Add a resource to the pipeline to receive deliveries.",
  [WebhookEndpointStatus.NOT_RUNNING]: "Senders retry until the pipeline starts.",
  [WebhookEndpointStatus.WAITING]: "Deliveries wait until the pipeline reads every resource.",
  [WebhookEndpointStatus.LIVE]: "Endpoints are live and accepting deliveries.",
};
