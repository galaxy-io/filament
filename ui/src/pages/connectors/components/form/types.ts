import type { JsonObject } from "@bufbuild/protobuf";

import type { ValidationError } from "@/gen/ingestion/v1/providers_pb";

export enum ConnectionFormPhase {
  IDLE = "IDLE",
  VALIDATING = "VALIDATING",
  VALIDATED = "VALIDATED",
  SUBMITTING = "SUBMITTING",
  ERROR = "ERROR",
}

export interface ConnectionFormState {
  name: string;
  config: JsonObject;
  phase: ConnectionFormPhase;
  validationErrors: ValidationError[];
  shouldShowErrors: boolean;
}
