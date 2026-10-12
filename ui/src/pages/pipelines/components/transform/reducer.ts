import { arrayMove } from "@dnd-kit/sortable";
import { match } from "ts-pattern";

import {
  type LoadAction,
  type MoveStepAction,
  type OpenStepAction,
  type PipelineTransformFieldsAction,
  PipelineTransformFieldsActionType,
  type RemoveStepAction,
  type SetDraftAction,
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
  resource,
  functionsByName,
}: LoadAction["payload"]): PipelineTransformFieldsState => ({
  steps: parseTransformDefinition(definition, resource, functionsByName),
  draft: null,
});

function load(_: PipelineTransformFieldsState, action: LoadAction): PipelineTransformFieldsState {
  return createInitialPipelineTransformFieldsState(action.payload);
}

function openNew(state: PipelineTransformFieldsState): PipelineTransformFieldsState {
  return {
    ...state,
    draft: { id: null, step: createTransformStep(TransformStepKind.COMPUTE) },
  };
}

function openStep(
  state: PipelineTransformFieldsState,
  action: OpenStepAction,
): PipelineTransformFieldsState {
  const step = state.steps.find((candidate) => candidate.id === action.payload);
  if (!step || step.kind === TransformStepKind.RAW) return state;
  return { ...state, draft: { id: step.id, step } };
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

function save(state: PipelineTransformFieldsState): PipelineTransformFieldsState {
  if (!state.draft) return state;
  return {
    steps: placeTransformDraft(state.steps, state.draft).steps,
    draft: null,
  };
}

function removeStep(
  state: PipelineTransformFieldsState,
  action: RemoveStepAction,
): PipelineTransformFieldsState {
  return {
    steps: state.steps.filter((step) => step.id !== action.payload),
    draft: state.draft?.id === action.payload ? null : state.draft,
  };
}

function moveStep(
  state: PipelineTransformFieldsState,
  action: MoveStepAction,
): PipelineTransformFieldsState {
  const { from, to } = action.payload;
  if (from === to) return state;
  return { ...state, steps: arrayMove(state.steps, from, to) };
}

const pipelineTransformFieldsReducer = (
  state: PipelineTransformFieldsState,
  action: PipelineTransformFieldsAction,
): PipelineTransformFieldsState =>
  match(action)
    .with({ type: PipelineTransformFieldsActionType.LOAD }, (action) => load(state, action))
    .with({ type: PipelineTransformFieldsActionType.OPEN_NEW }, () => openNew(state))
    .with({ type: PipelineTransformFieldsActionType.OPEN_STEP }, (action) =>
      openStep(state, action),
    )
    .with({ type: PipelineTransformFieldsActionType.CANCEL }, () => cancel(state))
    .with({ type: PipelineTransformFieldsActionType.SET_DRAFT }, (action) =>
      setDraft(state, action),
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
