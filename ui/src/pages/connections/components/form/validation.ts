import type { ValidationError } from "@/gen/ingestion/v1/connectors_pb";

export const createRequiredFieldsValidationErrorMap = (
  errors: ValidationError[],
): Map<ValidationError["field"], ValidationError["message"]> => {
  return new Map(errors.map((e) => [e.field, e.message]));
};
