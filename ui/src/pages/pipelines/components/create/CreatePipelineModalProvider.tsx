import {
  createContext,
  type Dispatch,
  type FC,
  type PropsWithChildren,
  useContext,
  useMemo,
  useReducer,
} from "react";

import { match } from "ts-pattern";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import {
  type AddNotifierAction,
  type AddResourceAction,
  type CreatePipelineModalAction,
  CreatePipelineModalActionType,
  type GoToStepAction,
  type RemoveNotifierAction,
  type SelectSourceAction,
  type SetActiveSinkAction,
  type SetDescriptionAction,
  type SetExecutionModeAction,
  type SetNameAction,
  type SetNodeConfigAction,
  type SetResourceCursorAction,
  type SetResourceReadModeAction,
  type SetResourceSelectionAction,
  type SetScheduleAction,
  type SetSinkWriteModeAction,
  type SetSubmittingAction,
  type SetWorkerConfigurationAction,
  type ToggleSinkAction,
  type UpdateNotifierAction,
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
  CreatePipelineModalStep,
} from "@/pages/pipelines/components/create/types";
import { createInitialCreatePipelineModalState } from "@/pages/pipelines/components/create/utils";
import { isPipelineNotifierValid } from "@/pages/pipelines/components/notifier/utils";
import { formatPipelineScheduleSummary } from "@/pages/pipelines/components/schedule/utils";
import { parseWorkerConfiguration } from "@/pages/pipelines/components/worker/utils";
import { getSupportedExecutionModes } from "@/pages/pipelines/utils";

import { getNameError, isNameValid } from "@/utils/validation";

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
      selectSource: (payload: SelectSourceAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.SELECT_SOURCE, payload }),
      toggleSink: (payload: ToggleSinkAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.TOGGLE_SINK, payload }),
      setActiveSink: (payload: SetActiveSinkAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.SET_ACTIVE_SINK, payload }),
      setExecutionMode: (payload: SetExecutionModeAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.SET_EXECUTION_MODE, payload }),
      addResource: (payload: AddResourceAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.ADD_RESOURCE, payload }),
      setResourceSelection: (payload: SetResourceSelectionAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.SET_RESOURCE_SELECTION, payload }),
      setResourceReadMode: (payload: SetResourceReadModeAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.SET_RESOURCE_READ_MODE, payload }),
      setResourceCursor: (payload: SetResourceCursorAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.SET_RESOURCE_CURSOR, payload }),
      setSinkWriteMode: (payload: SetSinkWriteModeAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.SET_SINK_WRITE_MODE, payload }),
      setNodeConfig: (payload: SetNodeConfigAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.SET_NODE_CONFIG, payload }),
      setName: (payload: SetNameAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.SET_NAME, payload }),
      setDescription: (payload: SetDescriptionAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.SET_DESCRIPTION, payload }),
      setSchedule: (payload: SetScheduleAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.SET_SCHEDULE, payload }),
      addNotifier: (payload: AddNotifierAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.ADD_NOTIFIER, payload }),
      updateNotifier: (payload: UpdateNotifierAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.UPDATE_NOTIFIER, payload }),
      removeNotifier: (payload: RemoveNotifierAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.REMOVE_NOTIFIER, payload }),
      setWorkerConfiguration: (payload: SetWorkerConfigurationAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.SET_WORKER_CONFIGURATION, payload }),
      goToStep: (payload: GoToStepAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.GO_TO_STEP, payload }),
      goBack: () => dispatch({ type: CreatePipelineModalActionType.GO_BACK }),
      goNext: () => dispatch({ type: CreatePipelineModalActionType.GO_NEXT }),
      setSubmitting: (payload: SetSubmittingAction["payload"]) =>
        dispatch({ type: CreatePipelineModalActionType.SET_SUBMITTING, payload }),
    }),
    [dispatch],
  );
};

interface CreatePipelineModalProviderProps {
  isOpen: boolean;
}

const CreatePipelineModalProvider: FC<PropsWithChildren<CreatePipelineModalProviderProps>> = ({
  isOpen,
  children,
}) => {
  const [state, dispatch] = useReducer(
    createPipelineModalReducer,
    undefined,
    createInitialCreatePipelineModalState,
  );

  const {
    rowsBySink,
    sinks,
    replication,
    hasReadLevers,
    issuesBySink,
    selectedCountBySink,
    isLoading,
    isValidating,
    discoverError,
  } = useCreatePipelineResources(state, isOpen);

  const value = useMemo<CreatePipelineModalContextValue>(() => {
    const supportedExecutionModes = getSupportedExecutionModes(state.sourceConnection);
    const isContinuous = state.executionMode === ExecutionMode.CONTINUOUS;

    const blockingMessages = sinks.flatMap((sink) =>
      (issuesBySink[sink.connection.id] ?? []).map((message) =>
        sinks.length > 1 ? `[Sink: ${sink.connection.name}] ${message}` : message,
      ),
    );

    const effectiveName = state.isNameTouched
      ? state.name
      : getDefaultPipelineName(state.sourceConnection, state.sinkConnections);

    const isScheduleValid =
      isContinuous ||
      !state.schedule.isEnabled ||
      formatPipelineScheduleSummary(state.schedule) !== null;
    const isNotifiersValid = state.notifiers.every(isPipelineNotifierValid);
    const workerConfigurationError = parseWorkerConfiguration(state.workerConfiguration).error;
    const isConnectionsValid =
      state.sinkConnections.length > 0 && supportedExecutionModes.length > 0;
    const isResourcesValid = !isValidating && !blockingMessages.length;

    const isNextDisabled = match(state.step)
      .with(CreatePipelineModalStep.CONNECTIONS, () => !isConnectionsValid)
      .with(CreatePipelineModalStep.RESOURCES, () => !isResourcesValid)
      .with(
        CreatePipelineModalStep.DELIVERY,
        () =>
          !isResourcesValid || !isScheduleValid || !isNotifiersValid || !!workerConfigurationError,
      )
      .with(
        CreatePipelineModalStep.DETAILS,
        () => !isResourcesValid || !isScheduleValid || !isNameValid(effectiveName),
      )
      .exhaustive();

    const activeSinkId = sinks.some((sink) => sink.connection.id === state.activeSinkId)
      ? state.activeSinkId
      : (sinks[0]?.connection.id ?? "");

    const stepIndex = CREATE_PIPELINE_MODAL_STEP_ORDER.indexOf(state.step);
    const blockingHints = blockingMessages.length
      ? blockingMessages
      : state.step === CreatePipelineModalStep.DELIVERY && isScheduleValid && !isNotifiersValid
        ? ["Complete the notifiers to continue"]
        : state.step === CreatePipelineModalStep.DELIVERY &&
            isScheduleValid &&
            workerConfigurationError
          ? ["Fix the worker configuration to continue"]
          : [CREATE_PIPELINE_MODAL_STEP_TO_HINT_MAP[state.step]];

    return {
      ...state,
      supportedExecutionModes,
      hasReadLevers,
      activeSinkId,
      rowsBySink,
      sinks,
      replication,
      issuesBySink,
      selectedCountBySink,
      isLoading,
      discoverError,
      effectiveName,
      nameError: getNameError(effectiveName, state.isNameTouched) ?? undefined,
      workerConfigurationError,
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
    hasReadLevers,
    issuesBySink,
    selectedCountBySink,
    isLoading,
    isValidating,
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
