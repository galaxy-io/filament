import { createContext, type PropsWithChildren, useContext, useReducer } from "react";

import type { ConnectionFormAction } from "@/pages/connectors/components/form/actions";
import connectionFormReducer from "@/pages/connectors/components/form/reducer";
import {
  ConnectionFormPhase,
  type ConnectionFormState,
} from "@/pages/connectors/components/form/types";

export type ConnectionFormContextShape = {
  state: ConnectionFormState;
  dispatch: React.Dispatch<ConnectionFormAction>;
};

export function createInitialState(
  initialState: Pick<ConnectionFormState, "name" | "config">,
): ConnectionFormState {
  return {
    name: initialState.name,
    config: initialState.config,
    phase: ConnectionFormPhase.IDLE,
    validationErrors: [],
    shouldShowErrors: false,
  };
}

export const ConnectionFormContext = createContext<ConnectionFormContextShape | null>(null);
ConnectionFormContext.displayName = "ConnectionFormContext";

export function useConnectionFormContext() {
  const context = useContext(ConnectionFormContext);
  if (!context) {
    throw new Error("useConnectionFormContext must be used within ConnectionFormProvider");
  }
  return context;
}

interface ConnectionFormProviderProps {
  initialState: Pick<ConnectionFormState, "name" | "config">;
}

const ConnectionFormProvider = ({
  children,
  initialState,
}: PropsWithChildren<ConnectionFormProviderProps>) => {
  const [state, dispatch] = useReducer(connectionFormReducer, initialState, createInitialState);

  return (
    <ConnectionFormContext.Provider value={{ state, dispatch }}>
      {children}
    </ConnectionFormContext.Provider>
  );
};

export default ConnectionFormProvider;
