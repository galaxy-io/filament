import {
  createContext,
  type Dispatch,
  type FC,
  type PropsWithChildren,
  useContext,
  useMemo,
  useReducer,
} from "react";

import {
  type ConnectionFormAction,
  ConnectionFormActionType,
  type SetConfigFieldAction,
  type SetNameAction,
  type SetPhaseAction,
  type SetShouldShowErrorsAction,
  type SetValidationErrorsAction,
} from "@/pages/connections/components/form/actions";
import connectionFormReducer from "@/pages/connections/components/form/reducer";
import {
  ConnectionFormPhase,
  type ConnectionFormState,
} from "@/pages/connections/components/form/types";

export const createInitialState = (
  initialState: Pick<ConnectionFormState, "name" | "config">,
): ConnectionFormState => {
  return {
    name: initialState.name,
    config: initialState.config,
    phase: ConnectionFormPhase.IDLE,
    validationErrors: [],
    shouldShowErrors: false,
  };
};

const ConnectionFormStateContext = createContext<ConnectionFormState | null>(null);
ConnectionFormStateContext.displayName = "ConnectionFormStateContext";

const ConnectionFormDispatchContext = createContext<Dispatch<ConnectionFormAction> | null>(null);
ConnectionFormDispatchContext.displayName = "ConnectionFormDispatchContext";

export const useConnectionFormState = () => {
  const state = useContext(ConnectionFormStateContext);
  if (!state) {
    throw new Error("useConnectionFormState must be used within ConnectionFormProvider");
  }
  return state;
};

const useConnectionFormDispatch = () => {
  const dispatch = useContext(ConnectionFormDispatchContext);
  if (!dispatch) {
    throw new Error("useConnectionFormDispatch must be used within ConnectionFormProvider");
  }
  return dispatch;
};

export const useConnectionFormActions = () => {
  const dispatch = useConnectionFormDispatch();

  return useMemo(
    () => ({
      setName: (payload: SetNameAction["payload"]) =>
        dispatch({ type: ConnectionFormActionType.SET_NAME, payload }),
      setConfigField: (payload: SetConfigFieldAction["payload"]) =>
        dispatch({ type: ConnectionFormActionType.SET_CONFIG_FIELD, payload }),
      setPhase: (payload: SetPhaseAction["payload"]) =>
        dispatch({ type: ConnectionFormActionType.SET_PHASE, payload }),
      setValidationErrors: (payload: SetValidationErrorsAction["payload"]) =>
        dispatch({ type: ConnectionFormActionType.SET_VALIDATION_ERRORS, payload }),
      setShouldShowErrors: (payload: SetShouldShowErrorsAction["payload"]) =>
        dispatch({ type: ConnectionFormActionType.SET_SHOULD_SHOW_ERRORS, payload }),
    }),
    [dispatch],
  );
};

interface ConnectionFormProviderProps {
  initialState: Pick<ConnectionFormState, "name" | "config">;
}

const ConnectionFormProvider: FC<PropsWithChildren<ConnectionFormProviderProps>> = ({
  children,
  initialState,
}) => {
  const [state, dispatch] = useReducer(connectionFormReducer, initialState, createInitialState);

  return (
    <ConnectionFormDispatchContext.Provider value={dispatch}>
      <ConnectionFormStateContext.Provider value={state}>
        {children}
      </ConnectionFormStateContext.Provider>
    </ConnectionFormDispatchContext.Provider>
  );
};

export default ConnectionFormProvider;
