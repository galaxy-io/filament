import { arrayMove } from "@dnd-kit/sortable";
import { match } from "ts-pattern";

import {
  type LoadAction,
  type MoveStepAction,
  type OpenNewAction,
  type OpenStepAction,
  type PipelineTransformFieldsAction,
  PipelineTransformFieldsActionType,
  type RemoveStepAction,
  type SetDraftAction,
  type SetResourceAction,
} from "@/pages/pipelines/components/transform/actions";
import { parseTransformDefinition } from "@/pages/pipelines/components/transform/grammar/parse";
import {
  type PipelineTransformFieldsState,
  TransformStepKind,
} from "@/pages/pipelines/components/transform/types";
import {
  createTransformStep,
  placeTransformDraft,
} from "@/pages/pipelines/components/transform/utils";

export const createInitialPipelineTransformFieldsState = ({
  definition,
  resources,
  functionsByName,
}: LoadAction["payload"]): PipelineTransformFieldsState => ({
  stepsByResource: parseTransformDefinition(definition, resources, functionsByName),
  draft: null,
});

function load(_: PipelineTransformFieldsState, action: LoadAction): PipelineTransformFieldsState {
  return createInitialPipelineTransformFieldsState(action.payload);
}

function openNew(
  state: PipelineTransformFieldsState,
  { payload: { resources } }: OpenNewAction,
): PipelineTransformFieldsState {
  return {
    ...state,
    draft: {
      resource: resources.length === 1 ? resources[0] : "",
      id: null,
      step: createTransformStep(TransformStepKind.COMPUTE),
    },
  };
}

function openStep(
  state: PipelineTransformFieldsState,
  action: OpenStepAction,
): PipelineTransformFieldsState {
  for (const [resource, steps] of state.stepsByResource) {
    const step = steps.find((candidate) => candidate.id === action.payload);
    if (!step || step.kind === TransformStepKind.RAW) continue;
    return { ...state, draft: { resource, id: step.id, step } };
  }
  return state;
}

function cancel(state: PipelineTransformFieldsState): PipelineTransformFieldsState {
  return { ...state, draft: null };
}

function setDraft(
  state: PipelineTransformFieldsState,
  action: SetDraftAction,
): PipelineTransformFieldsState {
  if (!state.draft) return state;
  return { ...state, draft: { ...state.draft, step: action.payload } };
}

function setResource(
  state: PipelineTransformFieldsState,
  action: SetResourceAction,
): PipelineTransformFieldsState {
  if (!state.draft) return state;
  return {
    ...state,
    draft: {
      ...state.draft,
      resource: action.payload,
      step: createTransformStep(state.draft.step.kind, "", state.draft.step.id),
    },
  };
}

function save(state: PipelineTransformFieldsState): PipelineTransformFieldsState {
  if (!state.draft) return state;
  return {
    stepsByResource: placeTransformDraft(state.stepsByResource, state.draft).stepsByResource,
    draft: null,
  };
}

function removeStep(
  state: PipelineTransformFieldsState,
  action: RemoveStepAction,
): PipelineTransformFieldsState {
  const stepsByResource = new Map(
    [...state.stepsByResource].map(([resource, steps]) => [
      resource,
      steps.filter((step) => step.id !== action.payload),
    ]),
  );
  return {
    stepsByResource,
    draft: state.draft?.id === action.payload ? null : state.draft,
  };
}

function moveStep(
  state: PipelineTransformFieldsState,
  action: MoveStepAction,
): PipelineTransformFieldsState {
  const { resource, from, to } = action.payload;
  const steps = state.stepsByResource.get(resource);
  if (!steps || from === to) return state;
  const stepsByResource = new Map(state.stepsByResource);
  stepsByResource.set(resource, arrayMove(steps, from, to));
  return { ...state, stepsByResource };
}

const pipelineTransformFieldsReducer = (
  state: PipelineTransformFieldsState,
  action: PipelineTransformFieldsAction,
): PipelineTransformFieldsState =>
  match(action)
    .with({ type: PipelineTransformFieldsActionType.LOAD }, (action) => load(state, action))
    .with({ type: PipelineTransformFieldsActionType.OPEN_NEW }, (action) => openNew(state, action))
    .with({ type: PipelineTransformFieldsActionType.OPEN_STEP }, (action) =>
      openStep(state, action),
    )
    .with({ type: PipelineTransformFieldsActionType.CANCEL }, () => cancel(state))
    .with({ type: PipelineTransformFieldsActionType.SET_DRAFT }, (action) =>
      setDraft(state, action),
    )
    .with({ type: PipelineTransformFieldsActionType.SET_RESOURCE }, (action) =>
      setResource(state, action),
    )
    .with({ type: PipelineTransformFieldsActionType.SAVE }, () => save(state))
    .with({ type: PipelineTransformFieldsActionType.REMOVE_STEP }, (action) =>
      removeStep(state, action),
    )
    .with({ type: PipelineTransformFieldsActionType.MOVE_STEP }, (action) =>
      moveStep(state, action),
    )
    .exhaustive();

export default pipelineTransformFieldsReducer;
