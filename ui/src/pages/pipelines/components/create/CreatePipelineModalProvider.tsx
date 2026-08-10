import {
  createContext,
  type Dispatch,
  type PropsWithChildren,
  useContext,
  useMemo,
  useReducer,
} from "react";

import { match } from "ts-pattern";

import type { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { getNameError, isNameValid } from "@/pages/connectors/components/form/validation";
import {
  type CreatePipelineModalAction,
  CreatePipelineModalActionType,
} from "@/pages/pipelines/components/create/actions";
import {
  CREATE_PIPELINE_MODAL_STEP_ORDER,
  CREATE_PIPELINE_MODAL_STEP_TO_HINT_MAP,
} from "@/pages/pipelines/components/create/constants";
import { useCreatePipelineResources } from "@/pages/pipelines/components/create/hooks/useCreatePipelineResources";
import createPipelineModalReducer from "@/pages/pipelines/components/create/reducer";
import { getDefaultPipelineName } from "@/pages/pipelines/components/create/rows";
import {
  type CreatePipelineModalContextValue,
  type CreatePipelineModalState,
  CreatePipelineModalStep,
} from "@/pages/pipelines/components/create/types";
import { PIPELINE_SCHEDULE_DEFAULT_STATE } from "@/pages/pipelines/settings/constants";
import type { PipelineSettingsPageScheduleState } from "@/pages/pipelines/settings/types";
import { formatPipelineScheduleSummary } from "@/pages/pipelines/settings/utils";

const DEFAULT_STATE: CreatePipelineModalState = {
  step: CreatePipelineModalStep.CONNECTIONS,
  activeSinkId: "",
  sourceConnection: null,
  sinkConnections: [],
  resourceSelection: {},
  resourceReadModes: {},
  resourceCursors: {},
  sinkWriteModes: {},
  name: "",
  isNameTouched: false,
  description: "",
  schedule: PIPELINE_SCHEDULE_DEFAULT_STATE,
  isSubmitting: false,
};

const CreatePipelineModalStateContext = createContext<CreatePipelineModalContextValue | null>(null);
CreatePipelineModalStateContext.displayName = "CreatePipelineModalStateContext";

const CreatePipelineModalDispatchContext =
  createContext<Dispatch<CreatePipelineModalAction> | null>(null);
CreatePipelineModalDispatchContext.displayName = "CreatePipelineModalDispatchContext";

export const useCreatePipelineModalState = () => {
  const state = useContext(CreatePipelineModalStateContext);
  if (!state) {
    throw new Error("useCreatePipelineModalState must be used within CreatePipelineModalProvider");
  }
  return state;
};

const useCreatePipelineModalDispatch = () => {
  const dispatch = useContext(CreatePipelineModalDispatchContext);
  if (!dispatch) {
    throw new Error(
      "useCreatePipelineModalDispatch must be used within CreatePipelineModalProvider",
    );
  }
  return dispatch;
};

export const useCreatePipelineModalActions = () => {
  const dispatch = useCreatePipelineModalDispatch();

  return useMemo(
    () => ({
      selectSource: (connection: Connection) =>
        dispatch({ type: CreatePipelineModalActionType.SELECT_SOURCE, payload: connection }),
      toggleSink: (connection: Connection) =>
        dispatch({ type: CreatePipelineModalActionType.TOGGLE_SINK, payload: connection }),
      setActiveSink: (sinkId: string) =>
        dispatch({ type: CreatePipelineModalActionType.SET_ACTIVE_SINK, payload: sinkId }),
      openSinkResources: (sinkId: string) =>
        dispatch({ type: CreatePipelineModalActionType.OPEN_SINK_RESOURCES, payload: sinkId }),
      setResourceSelection: (
        sinkId: string,
        visibleNames: string[],
        selection: Record<string, boolean>,
      ) =>
        dispatch({
          type: CreatePipelineModalActionType.SET_RESOURCE_SELECTION,
          payload: { sinkId, visibleNames, selection },
        }),
      setResourceReadMode: (sinkId: string, resource: string, readMode: ReadMode) =>
        dispatch({
          type: CreatePipelineModalActionType.SET_RESOURCE_READ_MODE,
          payload: { sinkId, resource, readMode },
        }),
      setResourceCursor: (sinkId: string, resource: string, cursorField: string) =>
        dispatch({
          type: CreatePipelineModalActionType.SET_RESOURCE_CURSOR,
          payload: { sinkId, resource, cursorField },
        }),
      setSinkWriteMode: (sinkId: string, writeMode: WriteMode) =>
        dispatch({
          type: CreatePipelineModalActionType.SET_SINK_WRITE_MODE,
          payload: { sinkId, writeMode },
        }),
      setName: (name: string) =>
        dispatch({ type: CreatePipelineModalActionType.SET_NAME, payload: name }),
      setDescription: (description: string) =>
        dispatch({ type: CreatePipelineModalActionType.SET_DESCRIPTION, payload: description }),
      setSchedule: (schedule: Partial<PipelineSettingsPageScheduleState>) =>
        dispatch({ type: CreatePipelineModalActionType.SET_SCHEDULE, payload: schedule }),
      goToStep: (step: CreatePipelineModalStep) =>
        dispatch({ type: CreatePipelineModalActionType.GO_TO_STEP, payload: step }),
      goBack: () => dispatch({ type: CreatePipelineModalActionType.GO_BACK }),
      goNext: () => dispatch({ type: CreatePipelineModalActionType.GO_NEXT }),
      setSubmitting: (isSubmitting: boolean) =>
        dispatch({ type: CreatePipelineModalActionType.SET_SUBMITTING, payload: isSubmitting }),
    }),
    [dispatch],
  );
};

const CreatePipelineModalProvider = ({ children }: PropsWithChildren) => {
  const [state, dispatch] = useReducer(createPipelineModalReducer, DEFAULT_STATE);

  const {
    rowsBySink,
    sinks,
    replication,
    isCdc,
    blockingMessages,
    issuesBySink,
    selectedCountBySink,
    isLoading,
    discoverError,
  } = useCreatePipelineResources(state);

  const value = useMemo<CreatePipelineModalContextValue>(() => {
    const effectiveName = state.isNameTouched
      ? state.name
      : getDefaultPipelineName(state.sourceConnection, state.sinkConnections);

    const isScheduleValid =
      !state.schedule.isEnabled || formatPipelineScheduleSummary(state.schedule) !== null;
    const isConnectionsValid = !!state.sourceConnection && state.sinkConnections.length > 0;
    const isResourcesValid = !blockingMessages.length;

    const isNextDisabled = match(state.step)
      .with(CreatePipelineModalStep.CONNECTIONS, () => !isConnectionsValid)
      .with(CreatePipelineModalStep.RESOURCES, () => !isResourcesValid)
      .with(CreatePipelineModalStep.DELIVERY, () => !isScheduleValid)
      .with(CreatePipelineModalStep.DETAILS, () => !isNameValid(effectiveName))
      .exhaustive();

    const activeSinkId = sinks.some((sink) => sink.connection.id === state.activeSinkId)
      ? state.activeSinkId
      : (sinks[0]?.connection.id ?? "");

    const stepIndex = CREATE_PIPELINE_MODAL_STEP_ORDER.indexOf(state.step);
    const blockingHints = blockingMessages.length
      ? blockingMessages
      : [CREATE_PIPELINE_MODAL_STEP_TO_HINT_MAP[state.step]];

    return {
      ...state,
      activeSinkId,
      rowsBySink,
      sinks,
      replication,
      isCdc,
      issuesBySink,
      selectedCountBySink,
      isLoading,
      discoverError,
      effectiveName,
      nameError: getNameError(effectiveName, state.isNameTouched) ?? undefined,
      isNextDisabled,
      hints: isNextDisabled ? blockingHints : [],
      stepIndex,
      isBackVisible: stepIndex > 0,
      isLastStep: stepIndex === CREATE_PIPELINE_MODAL_STEP_ORDER.length - 1,
    };
  }, [
    state,
    rowsBySink,
    sinks,
    replication,
    isCdc,
    blockingMessages,
    issuesBySink,
    selectedCountBySink,
    isLoading,
    discoverError,
  ]);

  return (
    <CreatePipelineModalDispatchContext.Provider value={dispatch}>
      <CreatePipelineModalStateContext.Provider value={value}>
        {children}
      </CreatePipelineModalStateContext.Provider>
    </CreatePipelineModalDispatchContext.Provider>
  );
};

export default CreatePipelineModalProvider;
