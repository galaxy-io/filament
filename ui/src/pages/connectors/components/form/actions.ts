import type { JsonValue } from "@bufbuild/protobuf";

import type { ValidationError } from "@/gen/ingestion/v1/providers_pb";

import type { ConnectionFormPhase } from "@/pages/connectors/components/form/types";

export enum ConnectionFormActionType {
  SET_NAME = "SET_NAME",
  SET_CONFIG_FIELD = "SET_CONFIG_FIELD",
  SET_PHASE = "SET_PHASE",
  SET_VALIDATION_ERRORS = "SET_VALIDATION_ERRORS",
  SET_SHOULD_SHOW_ERRORS = "SET_SHOULD_SHOW_ERRORS",
}

export interface SetNameAction {
  type: ConnectionFormActionType.SET_NAME;
  payload: string;
}

export interface SetConfigFieldAction {
  type: ConnectionFormActionType.SET_CONFIG_FIELD;
  payload: { field: string; value: JsonValue };
}

export interface SetPhaseAction {
  type: ConnectionFormActionType.SET_PHASE;
  payload: ConnectionFormPhase;
}

export interface SetValidationErrorsAction {
  type: ConnectionFormActionType.SET_VALIDATION_ERRORS;
  payload: ValidationError[];
}

export interface SetShouldShowErrorsAction {
  type: ConnectionFormActionType.SET_SHOULD_SHOW_ERRORS;
  payload: boolean;
}

export type ConnectionFormAction =
  | SetNameAction
  | SetConfigFieldAction
  | SetPhaseAction
  | SetValidationErrorsAction
  | SetShouldShowErrorsAction;
