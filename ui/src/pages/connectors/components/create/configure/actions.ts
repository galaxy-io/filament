import type { JsonValue } from "@bufbuild/protobuf";

import type { FieldType } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec, ValidationError } from "@/gen/ingestion/v1/providers_pb";

import type { CreateConnectionPhase } from "./types";

export enum CreateConnectionConfigureActionType {
  SET_CONNECTOR = "SET_CONNECTOR",
  SET_CONNECTION_NAME = "SET_CONNECTION_NAME",
  SET_PHASE = "SET_PHASE",
  SET_FIELD_VALUE = "SET_FIELD_VALUE",
  SET_VALIDATION_ERRORS = "SET_VALIDATION_ERRORS",
  SET_ERROR = "SET_ERROR",
  SET_SHOULD_SHOW_ERRORS = "SET_SHOULD_SHOW_ERRORS",
  RESET = "RESET",
}

export interface SetConnectorAction {
  type: CreateConnectionConfigureActionType.SET_CONNECTOR;
  payload: ConnectorSpec;
}

export interface SetConnectionNameAction {
  type: CreateConnectionConfigureActionType.SET_CONNECTION_NAME;
  payload: string;
}

export interface SetPhaseAction {
  type: CreateConnectionConfigureActionType.SET_PHASE;
  payload: CreateConnectionPhase;
}

export interface SetFieldValueAction {
  type: CreateConnectionConfigureActionType.SET_FIELD_VALUE;
  payload: { field: string; value: JsonValue; fieldType: FieldType };
}

export interface SetValidationErrorsAction {
  type: CreateConnectionConfigureActionType.SET_VALIDATION_ERRORS;
  payload: ValidationError[];
}

export interface SetErrorAction {
  type: CreateConnectionConfigureActionType.SET_ERROR;
  payload: string | null;
}

export interface SetShouldShowErrorsAction {
  type: CreateConnectionConfigureActionType.SET_SHOULD_SHOW_ERRORS;
  payload: boolean;
}

export interface ResetAction {
  type: CreateConnectionConfigureActionType.RESET;
}

export type CreateConnectionConfigureAction =
  | SetConnectorAction
  | SetConnectionNameAction
  | SetPhaseAction
  | SetFieldValueAction
  | SetValidationErrorsAction
  | SetErrorAction
  | SetShouldShowErrorsAction
  | ResetAction;
