import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { ValidationError } from "@/gen/ingestion/v1/connectors_pb";

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
