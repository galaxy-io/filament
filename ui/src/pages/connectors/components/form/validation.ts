import type { ValidationError } from "@/gen/ingestion/v1/connectors_pb";

export function createRequiredFieldsValidationErrorMap(
  errors: ValidationError[],
): Map<ValidationError["field"], ValidationError["message"]> {
  return new Map(errors.map((e) => [e.field, e.message]));
}

export function isNameValid(name: string | undefined): boolean {
  return (name ?? "").trim().length > 0;
}

export function getNameError(name: string | undefined, shouldShowErrors: boolean): string | null {
  return shouldShowErrors && !isNameValid(name) ? "Name is required" : null;
}
