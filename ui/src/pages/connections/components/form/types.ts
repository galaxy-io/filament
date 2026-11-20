import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { ValidationError, WebhookSpec } from "@/gen/ingestion/v1/connectors_pb";

import type { WebhookIngestion } from "@/components/webhooks/types";

export enum ConnectionFormPhase {
  IDLE = "IDLE",
  VALIDATING = "VALIDATING",
  VALIDATED = "VALIDATED",
  SUBMITTING = "SUBMITTING",
  ERROR = "ERROR",
}

export interface ConnectionFormState {
  name: Connection["name"];
  config: NonNullable<Connection["config"]>;
  phase: ConnectionFormPhase;
  validationErrors: ValidationError[];
  shouldShowErrors: boolean;
}

export interface ConnectionFormSigningKeyReveal {
  connection: Connection;
  webhook: WebhookSpec;
  signingKey: string;
  deliveryUrl: string;
}

export enum ConnectionFormWebhookStep {
  REVEAL = "REVEAL",
  SECRET = "SECRET",
}

export type ConnectionFormWebhookSetup =
  | { step: ConnectionFormWebhookStep.REVEAL; reveal: ConnectionFormSigningKeyReveal }
  | { step: ConnectionFormWebhookStep.SECRET; connection: Connection; webhook: WebhookSpec };

export type ConnectionFormIngestion = WebhookIngestion.API | WebhookIngestion.WEBHOOKS;
