import type { CreateConnectionRequest } from "@/gen/ingestion/v1/connections_pb";
import type { ConnectorSpec, ValidationError } from "@/gen/ingestion/v1/providers_pb";

export enum CreateConnectionPhase {
  IDLE = "IDLE",
  VALIDATING = "VALIDATING",
  VALIDATED = "VALIDATED",
  CREATING = "CREATING",
  ERROR = "ERROR",
}

export type CreateConnectionConfigureState = {
  request: CreateConnectionRequest;
  connector: ConnectorSpec;
  phase: CreateConnectionPhase;
  validationErrors: ValidationError[];
  error: string | null;
  shouldShowErrors: boolean;
};
