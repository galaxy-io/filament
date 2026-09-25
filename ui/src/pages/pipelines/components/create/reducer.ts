import type { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import {
  type AddNotifierAction,
  type AddResourceAction,
  type CreatePipelineModalAction,
  CreatePipelineModalActionType,
  type GoToStepAction,
  type OpenSinkResourcesAction,
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
import { CREATE_PIPELINE_MODAL_STEP_ORDER } from "@/pages/pipelines/components/create/constants";
import {
  type CreatePipelineModalState,
  CreatePipelineModalStep,
} from "@/pages/pipelines/components/create/types";
import { getSupportedExecutionModes } from "@/pages/pipelines/utils";

function applyExecutionMode(
  state: CreatePipelineModalState,
  executionMode: ExecutionMode,
): CreatePipelineModalState {
  if (executionMode === state.executionMode) return state;
  return {
    ...state,
    executionMode,
    sinkConnections: state.sinkConnections.filter((sink) =>
      sink.executionModes.includes(executionMode),
    ),
    manualResources: [],
    resourceSelection: {},
    resourceReadModes: {},
    resourceCursors: {},
    sinkWriteModes: {},
  };
}

function reconcileExecutionMode(state: CreatePipelineModalState): CreatePipelineModalState {
  const supported = getSupportedExecutionModes(state.sourceConnection);
  if (supported.includes(state.executionMode)) return state;
  return applyExecutionMode(state, supported[0] ?? state.executionMode);
}

function selectSource(
  state: CreatePipelineModalState,
  action: SelectSourceAction,
): CreatePipelineModalState {
  return reconcileExecutionMode({
    ...state,
    sourceConnection: state.sourceConnection?.id === action.payload.id ? null : action.payload,
    manualResources: [],
    resourceSelection: {},
    resourceReadModes: {},
    resourceCursors: {},
  });
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

function setExecutionMode(
  state: CreatePipelineModalState,
  action: SetExecutionModeAction,
): CreatePipelineModalState {
  return applyExecutionMode(state, action.payload);
}

function addResource(
  state: CreatePipelineModalState,
  action: AddResourceAction,
): CreatePipelineModalState {
  const { sinkId, name } = action.payload;
  return {
    ...state,
    manualResources: state.manualResources.includes(name)
      ? state.manualResources
      : [...state.manualResources, name],
    resourceSelection: {
      ...state.resourceSelection,
      [sinkId]: { ...state.resourceSelection[sinkId], [name]: true },
    },
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

function setNodeConfig(
  state: CreatePipelineModalState,
  action: SetNodeConfigAction,
): CreatePipelineModalState {
  return {
    ...state,
    nodeConfigs: { ...state.nodeConfigs, [action.payload.connectionId]: action.payload.config },
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

function addNotifier(
  state: CreatePipelineModalState,
  action: AddNotifierAction,
): CreatePipelineModalState {
  return { ...state, notifiers: [...state.notifiers, action.payload] };
}

function updateNotifier(
  state: CreatePipelineModalState,
  action: UpdateNotifierAction,
): CreatePipelineModalState {
  return {
    ...state,
    notifiers: state.notifiers.map((notifier) =>
      notifier.id === action.payload.id ? { ...notifier, ...action.payload.partial } : notifier,
    ),
  };
}

function removeNotifier(
  state: CreatePipelineModalState,
  action: RemoveNotifierAction,
): CreatePipelineModalState {
  return {
    ...state,
    notifiers: state.notifiers.filter((notifier) => notifier.id !== action.payload),
  };
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
    case CreatePipelineModalActionType.SET_EXECUTION_MODE:
      return setExecutionMode(state, action);
    case CreatePipelineModalActionType.ADD_RESOURCE:
      return addResource(state, action);
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
    case CreatePipelineModalActionType.SET_NODE_CONFIG:
      return setNodeConfig(state, action);
    case CreatePipelineModalActionType.SET_NAME:
      return setName(state, action);
    case CreatePipelineModalActionType.SET_DESCRIPTION:
      return setDescription(state, action);
    case CreatePipelineModalActionType.SET_SCHEDULE:
      return setSchedule(state, action);
    case CreatePipelineModalActionType.ADD_NOTIFIER:
      return addNotifier(state, action);
    case CreatePipelineModalActionType.UPDATE_NOTIFIER:
      return updateNotifier(state, action);
    case CreatePipelineModalActionType.REMOVE_NOTIFIER:
      return removeNotifier(state, action);
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
