import type { ValidationError } from "@/gen/ingestion/v1/connectors_pb";

export const createRequiredFieldsValidationErrorMap = (
  errors: ValidationError[],
): Map<ValidationError["field"], ValidationError["message"]> => {
  return new Map(errors.map((e) => [e.field, e.message]));
};

export const isNameValid = (name: string | undefined): boolean => {
  return (name ?? "").trim().length > 0;
};

export const getNameError = (
  name: string | undefined,
  shouldShowErrors: boolean,
): string | null => {
  return shouldShowErrors && !isNameValid(name) ? "Name is required" : null;
};
