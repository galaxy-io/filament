import {
  type CreatePipelineModalAction,
  CreatePipelineModalActionType,
  type GoToStepAction,
  type OpenSinkResourcesAction,
  type SelectSourceAction,
  type SetActiveSinkAction,
  type SetDescriptionAction,
  type SetNameAction,
  type SetResourceCursorAction,
  type SetResourceReadModeAction,
  type SetResourceSelectionAction,
  type SetScheduleAction,
  type SetSinkWriteModeAction,
  type SetSubmittingAction,
  type SetWorkerConfigurationAction,
  type ToggleSinkAction,
} from "@/pages/pipelines/components/create/actions";
import { CREATE_PIPELINE_MODAL_STEP_ORDER } from "@/pages/pipelines/components/create/constants";
import {
  type CreatePipelineModalState,
  CreatePipelineModalStep,
} from "@/pages/pipelines/components/create/types";

function selectSource(
  state: CreatePipelineModalState,
  action: SelectSourceAction,
): CreatePipelineModalState {
  return {
    ...state,
    sourceConnection: state.sourceConnection?.id === action.payload.id ? null : action.payload,
    resourceSelection: {},
    resourceReadModes: {},
    resourceCursors: {},
  };
}

function toggleSink(
  state: CreatePipelineModalState,
  action: ToggleSinkAction,
): CreatePipelineModalState {
  return {
    ...state,
    sinkConnections: state.sinkConnections.some((sink) => sink.id === action.payload.id)
      ? state.sinkConnections.filter((sink) => sink.id !== action.payload.id)
      : [...state.sinkConnections, action.payload],
  };
}

function setActiveSink(
  state: CreatePipelineModalState,
  action: SetActiveSinkAction,
): CreatePipelineModalState {
  return { ...state, activeSinkId: action.payload };
}

function openSinkResources(
  state: CreatePipelineModalState,
  action: OpenSinkResourcesAction,
): CreatePipelineModalState {
  return { ...state, activeSinkId: action.payload, step: CreatePipelineModalStep.RESOURCES };
}

function setResourceSelection(
  state: CreatePipelineModalState,
  action: SetResourceSelectionAction,
): CreatePipelineModalState {
  const { sinkId, visibleNames, selection } = action.payload;
  return {
    ...state,
    resourceSelection: {
      ...state.resourceSelection,
      [sinkId]: {
        ...state.resourceSelection[sinkId],
        ...Object.fromEntries(visibleNames.map((name) => [name, !!selection[name]])),
      },
    },
  };
}

function setResourceReadMode(
  state: CreatePipelineModalState,
  action: SetResourceReadModeAction,
): CreatePipelineModalState {
  const { sinkId, resource, readMode } = action.payload;
  return {
    ...state,
    resourceReadModes: {
      ...state.resourceReadModes,
      [sinkId]: { ...state.resourceReadModes[sinkId], [resource]: readMode },
    },
  };
}

function setSinkWriteMode(
  state: CreatePipelineModalState,
  action: SetSinkWriteModeAction,
): CreatePipelineModalState {
  return {
    ...state,
    sinkWriteModes: { ...state.sinkWriteModes, [action.payload.sinkId]: action.payload.writeMode },
  };
}

function setResourceCursor(
  state: CreatePipelineModalState,
  action: SetResourceCursorAction,
): CreatePipelineModalState {
  const { sinkId, resource, cursorField } = action.payload;
  return {
    ...state,
    resourceCursors: {
      ...state.resourceCursors,
      [sinkId]: { ...state.resourceCursors[sinkId], [resource]: cursorField },
    },
  };
}

function setName(state: CreatePipelineModalState, action: SetNameAction): CreatePipelineModalState {
  return { ...state, name: action.payload, isNameTouched: true };
}

function setDescription(
  state: CreatePipelineModalState,
  action: SetDescriptionAction,
): CreatePipelineModalState {
  return { ...state, description: action.payload };
}

function setSchedule(
  state: CreatePipelineModalState,
  action: SetScheduleAction,
): CreatePipelineModalState {
  return { ...state, schedule: { ...state.schedule, ...action.payload } };
}

function setWorkerConfiguration(
  state: CreatePipelineModalState,
  action: SetWorkerConfigurationAction,
): CreatePipelineModalState {
  return { ...state, workerConfiguration: action.payload };
}

function goToStep(
  state: CreatePipelineModalState,
  action: GoToStepAction,
): CreatePipelineModalState {
  return { ...state, step: action.payload };
}

function goBack(state: CreatePipelineModalState): CreatePipelineModalState {
  const stepIndex = CREATE_PIPELINE_MODAL_STEP_ORDER.indexOf(state.step);
  return { ...state, step: CREATE_PIPELINE_MODAL_STEP_ORDER[stepIndex - 1] ?? state.step };
}

function goNext(state: CreatePipelineModalState): CreatePipelineModalState {
  const stepIndex = CREATE_PIPELINE_MODAL_STEP_ORDER.indexOf(state.step);
  return { ...state, step: CREATE_PIPELINE_MODAL_STEP_ORDER[stepIndex + 1] ?? state.step };
}

function setSubmitting(
  state: CreatePipelineModalState,
  action: SetSubmittingAction,
): CreatePipelineModalState {
  return { ...state, isSubmitting: action.payload };
}

const createPipelineModalReducer = (
  state: CreatePipelineModalState,
  action: CreatePipelineModalAction,
): CreatePipelineModalState => {
  switch (action.type) {
    case CreatePipelineModalActionType.SELECT_SOURCE:
      return selectSource(state, action);
    case CreatePipelineModalActionType.TOGGLE_SINK:
      return toggleSink(state, action);
    case CreatePipelineModalActionType.SET_ACTIVE_SINK:
      return setActiveSink(state, action);
    case CreatePipelineModalActionType.OPEN_SINK_RESOURCES:
      return openSinkResources(state, action);
    case CreatePipelineModalActionType.SET_RESOURCE_SELECTION:
      return setResourceSelection(state, action);
    case CreatePipelineModalActionType.SET_RESOURCE_READ_MODE:
      return setResourceReadMode(state, action);
    case CreatePipelineModalActionType.SET_RESOURCE_CURSOR:
      return setResourceCursor(state, action);
    case CreatePipelineModalActionType.SET_SINK_WRITE_MODE:
      return setSinkWriteMode(state, action);
    case CreatePipelineModalActionType.SET_NAME:
      return setName(state, action);
    case CreatePipelineModalActionType.SET_DESCRIPTION:
      return setDescription(state, action);
    case CreatePipelineModalActionType.SET_SCHEDULE:
      return setSchedule(state, action);
    case CreatePipelineModalActionType.SET_WORKER_CONFIGURATION:
      return setWorkerConfiguration(state, action);
    case CreatePipelineModalActionType.GO_TO_STEP:
      return goToStep(state, action);
    case CreatePipelineModalActionType.GO_BACK:
      return goBack(state);
    case CreatePipelineModalActionType.GO_NEXT:
      return goNext(state);
    case CreatePipelineModalActionType.SET_SUBMITTING:
      return setSubmitting(state, action);
  }
};

export default createPipelineModalReducer;
