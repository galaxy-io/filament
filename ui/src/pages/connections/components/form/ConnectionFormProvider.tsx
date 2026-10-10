import { createContext, type FC, type PropsWithChildren, useContext, useReducer } from "react";

import type { ConnectionFormAction } from "@/pages/connections/components/form/actions";
import connectionFormReducer from "@/pages/connections/components/form/reducer";
import {
  ConnectionFormPhase,
  type ConnectionFormState,
} from "@/pages/connections/components/form/types";

type ConnectionFormContextShape = {
  state: ConnectionFormState;
  dispatch: React.Dispatch<ConnectionFormAction>;
};

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

const ConnectionFormContext = createContext<ConnectionFormContextShape | null>(null);
ConnectionFormContext.displayName = "ConnectionFormContext";

export const useConnectionFormContext = () => {
  const context = useContext(ConnectionFormContext);
  if (!context) {
    throw new Error("useConnectionFormContext must be used within ConnectionFormProvider");
  }
  return context;
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
    <ConnectionFormContext.Provider value={{ state, dispatch }}>
      {children}
    </ConnectionFormContext.Provider>
  );
};

export default ConnectionFormProvider;
