import type { ConfigField } from "@/gen/ingestion/v1/common_pb";
import type { ValidationError } from "@/gen/ingestion/v1/connectors_pb";

import {
  ConnectionFormPhase,
  type ConnectionFormState,
} from "@/pages/connections/components/form/types";

export const createInitialConnectionFormState = (
  initialState: Pick<ConnectionFormState, "name" | "config">,
): ConnectionFormState => ({
  name: initialState.name,
  config: initialState.config,
  phase: ConnectionFormPhase.IDLE,
  validationErrors: [],
  shouldShowErrors: false,
});

export const getUnplacedValidationErrors = (
  errors: ValidationError[],
  fields: ConfigField[],
): ValidationError[] => {
  const fieldNames = new Set(fields.map((field) => field.name));
  return errors.filter((error) => !fieldNames.has(error.field.split(".")[0]));
};
