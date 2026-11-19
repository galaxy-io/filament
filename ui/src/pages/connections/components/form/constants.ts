import { WebhookIngestion } from "@/components/webhooks/types";

import type { ConnectionFormIngestion } from "@/pages/connections/components/form/types";

export const CONNECTION_FORM_INGESTION_ORDER: ConnectionFormIngestion[] = [
  WebhookIngestion.API,
  WebhookIngestion.WEBHOOKS,
];

export const CONNECTION_FORM_INGESTION_TO_LABEL_MAP: Record<ConnectionFormIngestion, string> = {
  [WebhookIngestion.API]: "API",
  [WebhookIngestion.WEBHOOKS]: "Webhooks",
};

export const CONNECTION_FORM_INGESTION_TO_TOOLTIP_MAP: Record<
  ConnectionFormIngestion,
  (connectorName: string) => string
> = {
  [WebhookIngestion.API]: (connectorName) =>
    `Pipelines read the ${connectorName} API on a schedule or on demand`,
  [WebhookIngestion.WEBHOOKS]: (connectorName) =>
    `${connectorName} sends events to Filament and a stream reads them as they arrive`,
};
