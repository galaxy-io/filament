import {
  createContext,
  type PropsWithChildren,
  useMemo,
  useReducer,
} from "react";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import type { CreateConnectionAction } from "@/pages/connectors/components/create/configure/actions";
import createConnectionReducer from "@/pages/connectors/components/create/configure/reducer";
import {
  type CreateConnectionConfigureState,
  CreateConnectionPhase,
} from "@/pages/connectors/components/create/configure/types";
import { createInitialCreateConnectionRequest } from "@/pages/connectors/components/create/configure/utils";

export interface CreateConnectionConfigureContextShape {
  state: CreateConnectionConfigureState;
  dispatch: React.Dispatch<CreateConnectionAction>;
}

export const CreateConnectionConfigureContext =
  createContext<CreateConnectionConfigureContextShape | null>(null);
CreateConnectionConfigureContext.displayName =
  "CreateConnectionConfigureContext";

interface CreateConnectionConfigureProviderProps {
  connector: ConnectorSpec;
}

const CreateConnectionConfigureProvider = ({
  children,
  connector,
}: PropsWithChildren<CreateConnectionConfigureProviderProps>) => {
  const initialState: CreateConnectionConfigureState = useMemo(
    () => ({
      request: createInitialCreateConnectionRequest(connector),
      connector,
      phase: CreateConnectionPhase.IDLE,
      validationErrors: [],
      error: null,
      shouldShowErrors: false,
    }),
    [connector],
  );

  const [state, dispatch] = useReducer(createConnectionReducer, initialState);

  return (
    <CreateConnectionConfigureContext.Provider value={{ state, dispatch }}>
      {children}
    </CreateConnectionConfigureContext.Provider>
  );
};

export default CreateConnectionConfigureProvider;
