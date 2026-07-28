import type { CreateConnectionRequest } from "@/gen/ingestion/v1/connections_pb";
import type { ValidationError } from "@/gen/ingestion/v1/providers_pb";

export enum CreateConnectionPhase {
  IDLE = "IDLE",
  VALIDATING = "VALIDATING",
  VALIDATED = "VALIDATED",
  CREATING = "CREATING",
  ERROR = "ERROR",
}

export interface CreateConnectionConfigureState {
  request: CreateConnectionRequest;
  phase: CreateConnectionPhase;
  validationErrors: ValidationError[];
  shouldShowErrors: boolean;
}
