import { createContext, type PropsWithChildren, useMemo, useReducer } from "react";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import type { CreateConnectionConfigureAction } from "./actions";
import createConnectionConfigureReducer from "./reducer";
import { type CreateConnectionConfigureState, CreateConnectionPhase } from "./types";
import { getConnectionScopedFields, getInitialFormValues } from "./utils";

export interface CreateConnectionConfigureContextShape {
  state: CreateConnectionConfigureState;
  dispatch: React.Dispatch<CreateConnectionConfigureAction>;
}

export const DEFAULT_STATE: CreateConnectionConfigureState = {
  connector: null,
  connectionName: "",
  phase: CreateConnectionPhase.CONFIGURE,
  formValues: {},
  secretValues: {},
  validationErrors: [],
  error: null,
  shouldShowErrors: false,
};

export const CreateConnectionConfigureContext =
  createContext<CreateConnectionConfigureContextShape | null>(null);
CreateConnectionConfigureContext.displayName = "CreateConnectionConfigureContext";

interface CreateConnectionConfigureProviderProps {
  connector: ConnectorSpec;
}

const CreateConnectionConfigureProvider = ({
  children,
  connector,
}: PropsWithChildren<CreateConnectionConfigureProviderProps>) => {
  const initialFormValues = useMemo(() => {
    const fields = getConnectionScopedFields(connector);
    return getInitialFormValues(fields);
  }, [connector]);

  const [state, dispatch] = useReducer(createConnectionConfigureReducer, {
    ...DEFAULT_STATE,
    connector,
    formValues: initialFormValues,
  });

  return (
    <CreateConnectionConfigureContext.Provider value={{ state, dispatch }}>
      {children}
    </CreateConnectionConfigureContext.Provider>
  );
};

export default CreateConnectionConfigureProvider;
