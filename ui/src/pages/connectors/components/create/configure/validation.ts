import type { CreateConnectionRequest } from "@/gen/ingestion/v1/connections_pb";
import type { ValidationError } from "@/gen/ingestion/v1/providers_pb";

export function createRequiredFieldsValidationErrorMap(
  errors: ValidationError[],
): Map<string, string> {
  return new Map(errors.map((e) => [e.field, e.message]));
}

export function isNameValid(name: CreateConnectionRequest["name"] | undefined): boolean {
  return (name ?? "").trim().length > 0;
}

export function getNameError(
  name: CreateConnectionRequest["name"] | undefined,
  shouldShowErrors: boolean,
): string | null {
  return shouldShowErrors && !isNameValid(name) ? "Name is required" : null;
}
