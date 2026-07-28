import { create } from "@bufbuild/protobuf";

import { CreateConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";

import {
  type CreateConnectionAction,
  CreateConnectionActionType,
  type SetPhaseAction,
  type SetRequestConfigFieldAction,
  type SetRequestNameAction,
  type SetShouldShowErrorsAction,
  type SetValidationErrorsAction,
} from "@/pages/connectors/components/create/configure/actions";
import {
  type CreateConnectionConfigureState,
  CreateConnectionPhase,
} from "@/pages/connectors/components/create/configure/types";

function resetPhaseOnEdit(phase: CreateConnectionPhase): CreateConnectionPhase {
  return phase === CreateConnectionPhase.VALIDATED || phase === CreateConnectionPhase.ERROR
    ? CreateConnectionPhase.IDLE
    : phase;
}

function setRequestName(
  state: CreateConnectionConfigureState,
  action: SetRequestNameAction,
): CreateConnectionConfigureState {
  return {
    ...state,
    request: create(CreateConnectionRequestSchema, {
      ...state.request,
      name: action.payload,
    }),
    phase: resetPhaseOnEdit(state.phase),
    shouldShowErrors: false,
  };
}

function setRequestConfigField(
  state: CreateConnectionConfigureState,
  action: SetRequestConfigFieldAction,
): CreateConnectionConfigureState {
  return {
    ...state,
    request: create(CreateConnectionRequestSchema, {
      ...state.request,
      config: {
        ...(state.request.config ?? {}),
        [action.payload.field]: action.payload.value,
      },
    }),
    phase: resetPhaseOnEdit(state.phase),
    shouldShowErrors: false,
  };
}

function setPhase(
  state: CreateConnectionConfigureState,
  action: SetPhaseAction,
): CreateConnectionConfigureState {
  return {
    ...state,
    phase: action.payload,
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

function setShouldShowErrors(
  state: CreateConnectionConfigureState,
  action: SetShouldShowErrorsAction,
): CreateConnectionConfigureState {
  return {
    ...state,
    shouldShowErrors: action.payload,
  };
}

const createConnectionReducer = (
  state: CreateConnectionConfigureState,
  action: CreateConnectionAction,
): CreateConnectionConfigureState => {
  switch (action.type) {
    case CreateConnectionActionType.SET_REQUEST_NAME:
      return setRequestName(state, action);
    case CreateConnectionActionType.SET_REQUEST_CONFIG_FIELD:
      return setRequestConfigField(state, action);
    case CreateConnectionActionType.SET_PHASE:
      return setPhase(state, action);
    case CreateConnectionActionType.SET_VALIDATION_ERRORS:
      return setValidationErrors(state, action);
    case CreateConnectionActionType.SET_SHOULD_SHOW_ERRORS:
      return setShouldShowErrors(state, action);
  }
};

export default createConnectionReducer;
