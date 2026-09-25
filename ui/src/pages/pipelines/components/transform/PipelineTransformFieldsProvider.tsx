import {
  createContext,
  type PropsWithChildren,
  useContext,
  useMemo,
  useReducer,
  useRef,
  useState,
} from "react";

import { create } from "@bufbuild/protobuf";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import { ListTransformFunctionsRequestSchema } from "@/gen/ingestion/v1/transformations_pb";

import {
  type PipelineTransformFieldsAction,
  PipelineTransformFieldsActionType,
} from "@/pages/pipelines/components/transform/actions";
import { serializeTransformDefinition } from "@/pages/pipelines/components/transform/grammar/serialize";
import pipelineTransformFieldsReducer, {
  createInitialPipelineTransformFieldsState,
} from "@/pages/pipelines/components/transform/reducer";
import type {
  PipelineTransformFieldsEditor,
  PipelineTransformFieldsEnvironment,
  PipelineTransformFieldsState,
  TransformDefinition,
  TransformEditableStep,
  TransformStep,
} from "@/pages/pipelines/components/transform/types";

import { useSuspenseListTransformFunctionsQuery } from "@/api/queries/transforms";

interface PipelineTransformFieldsActions {
  openNew: () => void;
  openStep: (id: TransformStep["id"]) => void;
  cancel: () => void;
  setDraft: (step: TransformEditableStep) => void;
  setResource: (resource: Resource["name"]) => void;
  save: () => void;
  removeStep: (id: TransformStep["id"]) => void;
  moveStep: (resource: Resource["name"], from: number, to: number) => void;
}

const PipelineTransformFieldsStateContext = createContext<PipelineTransformFieldsState | null>(
  null,
);
PipelineTransformFieldsStateContext.displayName = "PipelineTransformFieldsStateContext";

const PipelineTransformFieldsActionsContext = createContext<PipelineTransformFieldsActions | null>(
  null,
);
PipelineTransformFieldsActionsContext.displayName = "PipelineTransformFieldsActionsContext";

const PipelineTransformFieldsEnvironmentContext =
  createContext<PipelineTransformFieldsEnvironment | null>(null);
PipelineTransformFieldsEnvironmentContext.displayName = "PipelineTransformFieldsEnvironmentContext";

export const PipelineTransformFieldsEditorContext =
  createContext<PipelineTransformFieldsEditor | null>(null);
PipelineTransformFieldsEditorContext.displayName = "PipelineTransformFieldsEditorContext";

export const usePipelineTransformFieldsState = () => {
  const state = useContext(PipelineTransformFieldsStateContext);
  if (!state) {
    throw new Error(
      "usePipelineTransformFieldsState must be used within PipelineTransformFieldsProvider",
    );
  }
  return state;
};

export const usePipelineTransformFieldsActions = () => {
  const actions = useContext(PipelineTransformFieldsActionsContext);
  if (!actions) {
    throw new Error(
      "usePipelineTransformFieldsActions must be used within PipelineTransformFieldsProvider",
    );
  }
  return actions;
};

export const usePipelineTransformFieldsEnvironment = () => {
  const environment = useContext(PipelineTransformFieldsEnvironmentContext);
  if (!environment) {
    throw new Error(
      "usePipelineTransformFieldsEnvironment must be used within PipelineTransformFieldsProvider",
    );
  }
  return environment;
};

export const usePipelineTransformFieldsEditor = () => {
  const editor = useContext(PipelineTransformFieldsEditorContext);
  if (!editor) {
    throw new Error(
      "usePipelineTransformFieldsEditor must be used within PipelineTransformFieldsStepCard",
    );
  }
  return editor;
};

interface PipelineTransformFieldsProviderProps {
  definition: TransformDefinition | undefined;
  onChange: (definition: TransformDefinition | undefined) => void;
  resources: Resource["name"][];
  columnsByResource: Map<Resource["name"], ResourceColumn[]>;
  sourceConnectionId: Connection["id"];
  isReadOnly?: boolean;
}

const PipelineTransformFieldsProvider = ({
  definition,
  onChange,
  resources,
  columnsByResource,
  sourceConnectionId,
  isReadOnly = false,
  children,
}: PropsWithChildren<PipelineTransformFieldsProviderProps>) => {
  const { data: catalog } = useSuspenseListTransformFunctionsQuery({
    input: create(ListTransformFunctionsRequestSchema, {}),
  });
  const functionsByName = useMemo(
    () => new Map(catalog.functions.map((fn) => [fn.name, fn])),
    [catalog.functions],
  );
  const [state, dispatch] = useReducer(
    pipelineTransformFieldsReducer,
    { definition, resources, functionsByName },
    createInitialPipelineTransformFieldsState,
  );

  // Reload when the definition changes underneath us (not from our own emit)
  // or when the edge covers a different set of resources.
  const emitted = useRef(definition);
  const [loaded, setLoaded] = useState({ definition, resources });
  if (loaded.definition !== definition || loaded.resources !== resources) {
    setLoaded({ definition, resources });
    if (definition !== emitted.current || loaded.resources !== resources) {
      dispatch({
        type: PipelineTransformFieldsActionType.LOAD,
        payload: { definition, resources, functionsByName },
      });
    }
  }

  const actions = useMemo<PipelineTransformFieldsActions>(() => {
    const commit = (action: PipelineTransformFieldsAction) => {
      const next = pipelineTransformFieldsReducer(state, action);
      dispatch(action);
      const nextDefinition = serializeTransformDefinition(
        next.stepsByResource,
        catalog.grammarVersion,
        definition,
      );
      emitted.current = nextDefinition;
      onChange(nextDefinition);
    };
    return {
      openNew: () =>
        dispatch({ type: PipelineTransformFieldsActionType.OPEN_NEW, payload: { resources } }),
      openStep: (id) =>
        dispatch({ type: PipelineTransformFieldsActionType.OPEN_STEP, payload: id }),
      cancel: () => dispatch({ type: PipelineTransformFieldsActionType.CANCEL }),
      setDraft: (step) =>
        dispatch({ type: PipelineTransformFieldsActionType.SET_DRAFT, payload: step }),
      setResource: (resource) =>
        dispatch({ type: PipelineTransformFieldsActionType.SET_RESOURCE, payload: resource }),
      save: () => commit({ type: PipelineTransformFieldsActionType.SAVE }),
      removeStep: (id) =>
        commit({ type: PipelineTransformFieldsActionType.REMOVE_STEP, payload: id }),
      moveStep: (resource, from, to) =>
        commit({
          type: PipelineTransformFieldsActionType.MOVE_STEP,
          payload: { resource, from, to },
        }),
    };
  }, [state, catalog.grammarVersion, onChange, resources, definition]);

  const environment = useMemo<PipelineTransformFieldsEnvironment>(
    () => ({
      resources,
      columnsByResource,
      sourceConnectionId,
      functionsByName,
      grammarVersion: catalog.grammarVersion,
      isReadOnly,
    }),
    [
      resources,
      columnsByResource,
      sourceConnectionId,
      functionsByName,
      catalog.grammarVersion,
      isReadOnly,
    ],
  );

  return (
    <PipelineTransformFieldsEnvironmentContext.Provider value={environment}>
      <PipelineTransformFieldsActionsContext.Provider value={actions}>
        <PipelineTransformFieldsStateContext.Provider value={state}>
          {children}
        </PipelineTransformFieldsStateContext.Provider>
      </PipelineTransformFieldsActionsContext.Provider>
    </PipelineTransformFieldsEnvironmentContext.Provider>
  );
};

export default PipelineTransformFieldsProvider;
