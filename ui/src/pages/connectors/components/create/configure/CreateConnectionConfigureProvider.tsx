import { createContext, type PropsWithChildren, useReducer } from "react";

import { create } from "@bufbuild/protobuf";

import type { CreateConnectionAction } from "@/pages/connectors/components/create/configure/actions";
import createConnectionReducer from "@/pages/connectors/components/create/configure/reducer";
import {
  type CreateConnectionConfigureState,
  CreateConnectionPhase,
} from "@/pages/connectors/components/create/configure/types";

import { CreateConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

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
    connector,
    phase: CreateConnectionPhase.IDLE,
    validationErrors: [],
    error: null,
    shouldShowErrors: false,
  };
}

const DEFAULT_CONTEXT: CreateConnectionConfigureContextShape = {
  state: createInitialState({} as ConnectorSpec),
  dispatch: () => undefined,
};

export const CreateConnectionConfigureContext =
  createContext<CreateConnectionConfigureContextShape>(DEFAULT_CONTEXT);
CreateConnectionConfigureContext.displayName = "CreateConnectionConfigureContext";

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
