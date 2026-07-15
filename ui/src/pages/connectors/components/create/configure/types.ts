import type { CreateConnectionRequest } from "@/gen/ingestion/v1/connections_pb";
import type { ConnectorSpec, ValidationError } from "@/gen/ingestion/v1/providers_pb";

export enum CreateConnectionPhase {
  IDLE = "IDLE",
  VALIDATING = "VALIDATING",
  VALIDATED = "VALIDATED",
  CREATING = "CREATING",
  ERROR = "ERROR",
}

/**
 * State for the create connection form.
 */
export interface CreateConnectionConfigureState {
  /**
   * The connection request being built.
   */
  request: CreateConnectionRequest;

  /**
   * The selected connector spec.
   */
  connector: ConnectorSpec;

  /**
   * Current phase of the creation flow.
   */
  phase: CreateConnectionPhase;

  /**
   * Server-side validation errors.
   */
  validationErrors: ValidationError[];

  /**
   * General error message.
   */
  error: string | null;

  /**
   * Should validation errors be displayed?
   */
  shouldShowErrors: boolean;
}
