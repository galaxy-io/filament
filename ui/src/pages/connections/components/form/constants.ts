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
