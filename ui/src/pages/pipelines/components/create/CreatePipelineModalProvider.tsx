import {
  createContext,
  type Dispatch,
  type PropsWithChildren,
  useContext,
  useMemo,
  useReducer,
} from "react";

import { match } from "ts-pattern";

import { getNameError, isNameValid } from "@/pages/connectors/components/form/validation";
import type { CreatePipelineModalAction } from "@/pages/pipelines/components/create/actions";
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
import { WORKER_RESOURCES_DEFAULT_STATE } from "@/pages/pipelines/components/worker/WorkerResourcesFields";
import { PIPELINE_SCHEDULE_DEFAULT_STATE } from "@/pages/pipelines/settings/constants";
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
  workerResources: WORKER_RESOURCES_DEFAULT_STATE,
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
    isCdc,
    issuesBySink,
    selectedCountBySink,
    isLoading,
    discoverError,
  } = useCreatePipelineResources(state);

  const value = useMemo<CreatePipelineModalContextValue>(() => {
    const blockingMessages = sinks.flatMap((sink) =>
      (issuesBySink[sink.connection.id] ?? []).map((message) =>
        sinks.length > 1 ? `[Sink: ${sink.connection.name}] ${message}` : message,
      ),
    );

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
