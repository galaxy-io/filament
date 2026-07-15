import { CreateConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import {
  type CreateConnectionAction,
  CreateConnectionActionType,
  type SetErrorAction,
  type SetRequestConfigFieldAction,
  type SetRequestNameAction,
  type SetSecretValueAction,
  type SetShouldShowErrorsAction,
  type SetStepAction,
  type SetValidationErrorsAction,
} from "@/pages/connectors/components/create/configure/actions";
import {
  type CreateConnectionConfigureState,
  CreateConnectionPhase,
} from "@/pages/connectors/components/create/configure/types";
import { create } from "@bufbuild/protobuf";

function resetStepIfNeeded(step: CreateConnectionPhase): CreateConnectionPhase {
  return step === CreateConnectionPhase.VALIDATED ||
    step === CreateConnectionPhase.ERROR
    ? CreateConnectionPhase.IDLE
    : step;
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
    phase: resetStepIfNeeded(state.phase),
  };
}

function setRequestConfigField(
  state: CreateConnectionConfigureState,
  action: SetRequestConfigFieldAction,
): CreateConnectionConfigureState {
  const { field, value } = action.payload;
  return {
    ...state,
    request: create(CreateConnectionRequestSchema, {
      ...state.request,
      config: {
        ...(state.request.config ?? {}),
        [field]: value,
      },
    }),
    phase: resetStepIfNeeded(state.phase),
  };
}

function setSecretValue(
  state: CreateConnectionConfigureState,
  action: SetSecretValueAction,
): CreateConnectionConfigureState {
  const { field, value } = action.payload;
  return {
    ...state,
    request: create(CreateConnectionRequestSchema, {
      ...state.request,
      secretRefs: {
        ...(state.request.secretRefs ?? {}),
        [field]: value,
      },
    }),
    phase: resetStepIfNeeded(state.phase),
  };
}

function setStep(
  state: CreateConnectionConfigureState,
  action: SetStepAction,
): CreateConnectionConfigureState {
  return {
    ...state,
    phase: action.payload,
    error: action.payload === CreateConnectionPhase.ERROR ? state.error : null,
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

const createConnectionReducer = (
  state: CreateConnectionConfigureState,
  action: CreateConnectionAction,
): CreateConnectionConfigureState => {
  switch (action.type) {
    case CreateConnectionActionType.SET_REQUEST_NAME:
      return setRequestName(state, action);
    case CreateConnectionActionType.SET_REQUEST_CONFIG_FIELD:
      return setRequestConfigField(state, action);
    case CreateConnectionActionType.SET_SECRET_VALUE:
      return setSecretValue(state, action);
    case CreateConnectionActionType.SET_STEP:
      return setStep(state, action);
    case CreateConnectionActionType.SET_VALIDATION_ERRORS:
      return setValidationErrors(state, action);
    case CreateConnectionActionType.SET_ERROR:
      return setError(state, action);
    case CreateConnectionActionType.SET_SHOULD_SHOW_ERRORS:
      return setShouldShowErrors(state, action);
  }
};

export default createConnectionReducer;
