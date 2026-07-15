import { FieldType } from "@/gen/ingestion/v1/common_pb";

import {
  type CreateConnectionConfigureAction,
  CreateConnectionConfigureActionType,
  type SetConnectionNameAction,
  type SetConnectorAction,
  type SetErrorAction,
  type SetFieldValueAction,
  type SetPhaseAction,
  type SetShouldShowErrorsAction,
  type SetValidationErrorsAction,
} from "./actions";
import { DEFAULT_STATE } from "./CreateConnectionConfigureProvider";
import { type CreateConnectionConfigureState, CreateConnectionPhase } from "./types";

function setConnector(
  state: CreateConnectionConfigureState,
  action: SetConnectorAction,
): CreateConnectionConfigureState {
  return {
    ...state,
    connector: action.payload,
  };
}

function setConnectionName(
  state: CreateConnectionConfigureState,
  action: SetConnectionNameAction,
): CreateConnectionConfigureState {
  return {
    ...state,
    connectionName: action.payload,
  };
}

function setPhase(
  state: CreateConnectionConfigureState,
  action: SetPhaseAction,
): CreateConnectionConfigureState {
  return {
    ...state,
    phase: action.payload,
    error: action.payload === CreateConnectionPhase.ERROR ? state.error : null,
  };
}

function setFieldValue(
  state: CreateConnectionConfigureState,
  action: SetFieldValueAction,
): CreateConnectionConfigureState {
  const { field, value, fieldType } = action.payload;
  const resetPhase =
    state.phase === CreateConnectionPhase.VALIDATED || state.phase === CreateConnectionPhase.ERROR
      ? CreateConnectionPhase.CONFIGURE
      : state.phase;

  if (fieldType === FieldType.SECRET) {
    return {
      ...state,
      secretValues: {
        ...state.secretValues,
        [field]: value as string,
      },
      phase: resetPhase,
    };
  }

  return {
    ...state,
    formValues: {
      ...state.formValues,
      [field]: value,
    },
    phase: resetPhase,
  };
}

function setValidationErrors(
  state: CreateConnectionConfigureState,
  action: SetValidationErrorsAction,
): CreateConnectionConfigureState {
  return {
    ...state,
    validationErrors: action.payload,
  };
}

function setError(
  state: CreateConnectionConfigureState,
  action: SetErrorAction,
): CreateConnectionConfigureState {
  return {
    ...state,
    error: action.payload,
    phase: action.payload ? CreateConnectionPhase.ERROR : state.phase,
  };
}

function setShouldShowErrors(
  state: CreateConnectionConfigureState,
  action: SetShouldShowErrorsAction,
): CreateConnectionConfigureState {
  return {
    ...state,
    shouldShowErrors: action.payload,
  };
}

const createConnectionConfigureReducer = (
  state: CreateConnectionConfigureState,
  action: CreateConnectionConfigureAction,
): CreateConnectionConfigureState => {
  switch (action.type) {
    case CreateConnectionConfigureActionType.SET_CONNECTOR:
      return setConnector(state, action);
    case CreateConnectionConfigureActionType.SET_CONNECTION_NAME:
      return setConnectionName(state, action);
    case CreateConnectionConfigureActionType.SET_PHASE:
      return setPhase(state, action);
    case CreateConnectionConfigureActionType.SET_FIELD_VALUE:
      return setFieldValue(state, action);
    case CreateConnectionConfigureActionType.SET_VALIDATION_ERRORS:
      return setValidationErrors(state, action);
    case CreateConnectionConfigureActionType.SET_ERROR:
      return setError(state, action);
    case CreateConnectionConfigureActionType.SET_SHOULD_SHOW_ERRORS:
      return setShouldShowErrors(state, action);
    case CreateConnectionConfigureActionType.RESET:
      return DEFAULT_STATE;
  }
};

export default createConnectionConfigureReducer;
