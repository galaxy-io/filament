import { createContext, type PropsWithChildren, useContext, useReducer } from "react";

import { create } from "@bufbuild/protobuf";

import { CreateConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import type { CreateConnectionAction } from "@/pages/connectors/components/create/configure/actions";
import createConnectionReducer from "@/pages/connectors/components/create/configure/reducer";
import {
  type CreateConnectionConfigureState,
  CreateConnectionPhase,
} from "@/pages/connectors/components/create/configure/types";

export type CreateConnectionConfigureContextShape = {
  state: CreateConnectionConfigureState;
  dispatch: React.Dispatch<CreateConnectionAction>;
};

export function createInitialState(connector: ConnectorSpec): CreateConnectionConfigureState {
  return {
    request: create(CreateConnectionRequestSchema, {
      kind: connector.kind,
      connector: connector.name,
    }),
    phase: CreateConnectionPhase.IDLE,
    validationErrors: [],
    shouldShowErrors: false,
  };
}

export const CreateConnectionConfigureContext =
  createContext<CreateConnectionConfigureContextShape | null>(null);
CreateConnectionConfigureContext.displayName = "CreateConnectionConfigureContext";

export function useCreateConnectionContext() {
  const context = useContext(CreateConnectionConfigureContext);
  if (!context) {
    throw new Error(
      "useCreateConnectionContext must be used within CreateConnectionConfigureProvider",
    );
  }
  return context;
}

interface CreateConnectionConfigureProviderProps {
  connector: ConnectorSpec;
}

const CreateConnectionConfigureProvider = ({
  children,
  connector,
}: PropsWithChildren<CreateConnectionConfigureProviderProps>) => {
  const [state, dispatch] = useReducer(createConnectionReducer, connector, createInitialState);

  return (
    <CreateConnectionConfigureContext.Provider value={{ state, dispatch }}>
      {children}
    </CreateConnectionConfigureContext.Provider>
  );
};

export default CreateConnectionConfigureProvider;
