import {
  type ConnectionFormAction,
  ConnectionFormActionType,
  type SetConfigFieldAction,
  type SetNameAction,
  type SetPhaseAction,
  type SetShouldShowErrorsAction,
  type SetValidationErrorsAction,
} from "@/pages/connectors/components/form/actions";
import {
  ConnectionFormPhase,
  type ConnectionFormState,
} from "@/pages/connectors/components/form/types";

function resetPhaseOnEdit(phase: ConnectionFormPhase): ConnectionFormPhase {
  return phase === ConnectionFormPhase.VALIDATED || phase === ConnectionFormPhase.ERROR
    ? ConnectionFormPhase.IDLE
    : phase;
}

function setName(state: ConnectionFormState, action: SetNameAction): ConnectionFormState {
  return {
    ...state,
    name: action.payload,
    phase: resetPhaseOnEdit(state.phase),
    shouldShowErrors: false,
  };
}

function setConfigField(
  state: ConnectionFormState,
  action: SetConfigFieldAction,
): ConnectionFormState {
  return {
    ...state,
    config: {
      ...state.config,
      [action.payload.field]: action.payload.value,
    },
    phase: resetPhaseOnEdit(state.phase),
    shouldShowErrors: false,
  };
}

function setPhase(state: ConnectionFormState, action: SetPhaseAction): ConnectionFormState {
  return {
    ...state,
    phase: action.payload,
  };
}

function setValidationErrors(
  state: ConnectionFormState,
  action: SetValidationErrorsAction,
): ConnectionFormState {
  return {
    ...state,
    validationErrors: action.payload,
  };
}

function setShouldShowErrors(
  state: ConnectionFormState,
  action: SetShouldShowErrorsAction,
): ConnectionFormState {
  return {
    ...state,
    shouldShowErrors: action.payload,
  };
}

const connectionFormReducer = (
  state: ConnectionFormState,
  action: ConnectionFormAction,
): ConnectionFormState => {
  switch (action.type) {
    case ConnectionFormActionType.SET_NAME:
      return setName(state, action);
    case ConnectionFormActionType.SET_CONFIG_FIELD:
      return setConfigField(state, action);
    case ConnectionFormActionType.SET_PHASE:
      return setPhase(state, action);
    case ConnectionFormActionType.SET_VALIDATION_ERRORS:
      return setValidationErrors(state, action);
    case ConnectionFormActionType.SET_SHOULD_SHOW_ERRORS:
      return setShouldShowErrors(state, action);
  }
};

export default connectionFormReducer;
