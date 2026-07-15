import type { JsonValue } from "@bufbuild/protobuf";

import type { ValidationError } from "@/gen/ingestion/v1/providers_pb";

import type { CreateConnectionPhase } from "./types";

export enum CreateConnectionActionType {
  SET_REQUEST_NAME = "SET_REQUEST_NAME",
  SET_REQUEST_CONFIG_FIELD = "SET_REQUEST_CONFIG_FIELD",
  SET_PHASE = "SET_PHASE",
  SET_VALIDATION_ERRORS = "SET_VALIDATION_ERRORS",
  SET_ERROR = "SET_ERROR",
  SET_SHOULD_SHOW_ERRORS = "SET_SHOULD_SHOW_ERRORS",
}

export interface SetRequestNameAction {
  type: CreateConnectionActionType.SET_REQUEST_NAME;
  payload: string;
}

export interface SetRequestConfigFieldAction {
  type: CreateConnectionActionType.SET_REQUEST_CONFIG_FIELD;
  payload: { field: string; value: JsonValue };
}

export interface SetPhaseAction {
  type: CreateConnectionActionType.SET_PHASE;
  payload: CreateConnectionPhase;
}

export interface SetValidationErrorsAction {
  type: CreateConnectionActionType.SET_VALIDATION_ERRORS;
  payload: ValidationError[];
}

export interface SetErrorAction {
  type: CreateConnectionActionType.SET_ERROR;
  payload: string | null;
}

export interface SetShouldShowErrorsAction {
  type: CreateConnectionActionType.SET_SHOULD_SHOW_ERRORS;
  payload: boolean;
}

export type CreateConnectionAction =
  | SetRequestNameAction
  | SetRequestConfigFieldAction
  | SetPhaseAction
  | SetValidationErrorsAction
  | SetErrorAction
  | SetShouldShowErrorsAction;
