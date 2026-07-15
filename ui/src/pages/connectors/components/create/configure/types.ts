import type { JsonValue } from "@bufbuild/protobuf";

import type { ConnectorSpec, ValidationError } from "@/gen/ingestion/v1/providers_pb";

export enum CreateConnectionPhase {
  CONFIGURE = "CONFIGURE",
  VALIDATING = "VALIDATING",
  VALIDATED = "VALIDATED",
  CREATING = "CREATING",
  ERROR = "ERROR",
}

export interface CreateConnectionState {
  connectionName: string;
  shouldShowErrors: boolean;
  error: string | null;
}

export type CreateConnectionConfigureState = CreateConnectionState & {
  connector: ConnectorSpec | null;
  phase: CreateConnectionPhase;
  formValues: Record<string, JsonValue>;
  secretValues: Record<string, string>;
  validationErrors: ValidationError[];
};
