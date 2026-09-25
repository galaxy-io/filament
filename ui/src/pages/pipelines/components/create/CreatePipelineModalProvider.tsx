import {
  createContext,
  type Dispatch,
  type PropsWithChildren,
  useContext,
  useMemo,
  useReducer,
} from "react";

import { match } from "ts-pattern";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import { getNameError, isNameValid } from "@/pages/connectors/components/form/validation";
import type { CreatePipelineModalAction } from "@/pages/pipelines/components/create/actions";
import {
  CREATE_PIPELINE_MODAL_NO_EXECUTION_MODE_HINT,
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
import { isPipelineNotifierValid } from "@/pages/pipelines/components/notifier/utils";
import {
  DEFAULT_WORKER_CONFIGURATION_TEXT,
  parseWorkerConfiguration,
} from "@/pages/pipelines/components/worker/utils";
import { PIPELINE_SCHEDULE_DEFAULT_STATE } from "@/pages/pipelines/settings/constants";
import { PipelineScheduleFrequency } from "@/pages/pipelines/settings/types";
import { formatPipelineScheduleSummary } from "@/pages/pipelines/settings/utils";
import { getSupportedExecutionModes } from "@/pages/pipelines/utils";

const DEFAULT_STATE: CreatePipelineModalState = {
  executionMode: ExecutionMode.BOUNDED,
  manualResources: [],
  step: CreatePipelineModalStep.CONNECTIONS,
  activeSinkId: "",
  sourceConnection: null,
  sinkConnections: [],
  resourceSelection: {},
  resourceReadModes: {},
  resourceCursors: {},
  sinkWriteModes: {},
  nodeConfigs: {},
  name: "",
  isNameTouched: false,
  description: "",
  schedule: {
    ...PIPELINE_SCHEDULE_DEFAULT_STATE,
    isEnabled: true,
    frequency: PipelineScheduleFrequency.HOURLY,
  },
  notifiers: [],
  workerConfiguration: DEFAULT_WORKER_CONFIGURATION_TEXT,
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

export const useCreatePipelineModalDispatch = () => {
  const dispatch = useContext(CreatePipelineModalDispatchContext);
  if (!dispatch) {
    throw new Error(
      "useCreatePipelineModalDispatch must be used within CreatePipelineModalProvider",
    );
  }
  return dispatch;
};

const CreatePipelineModalProvider = ({ children }: PropsWithChildren) => {
  const [state, dispatch] = useReducer(createPipelineModalReducer, DEFAULT_STATE);

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
  } = useCreatePipelineResources(state);

  const value = useMemo<CreatePipelineModalContextValue>(() => {
    const supportedExecutionModes = getSupportedExecutionModes(
      state.sourceConnection,
      state.sinkConnections,
    );
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
    const blockingHints =
      state.step === CreatePipelineModalStep.CONNECTIONS &&
      state.sourceConnection &&
      state.sinkConnections.length > 0
        ? [CREATE_PIPELINE_MODAL_NO_EXECUTION_MODE_HINT]
        : blockingMessages.length
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
