import type { Resource } from "@/gen/ingestion/v1/connectors_pb";
import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import type {
  TransformDefinition,
  TransformEditableStep,
  TransformStep,
} from "@/pages/pipelines/components/transform/types";

export enum PipelineTransformFieldsActionType {
  LOAD = "LOAD",
  OPEN_NEW = "OPEN_NEW",
  OPEN_STEP = "OPEN_STEP",
  CANCEL = "CANCEL",
  SET_DRAFT = "SET_DRAFT",
  SET_RESOURCE = "SET_RESOURCE",
  SAVE = "SAVE",
  REMOVE_STEP = "REMOVE_STEP",
  MOVE_STEP = "MOVE_STEP",
}

export interface LoadAction {
  type: PipelineTransformFieldsActionType.LOAD;
  payload: {
    definition: TransformDefinition | undefined;
    resources: Resource["name"][];
    functionsByName: Map<TransformFunction["name"], TransformFunction>;
  };
}

export interface OpenNewAction {
  type: PipelineTransformFieldsActionType.OPEN_NEW;
}

export interface OpenStepAction {
  type: PipelineTransformFieldsActionType.OPEN_STEP;
  payload: TransformStep["id"];
}

export interface CancelAction {
  type: PipelineTransformFieldsActionType.CANCEL;
}

export interface SetDraftAction {
  type: PipelineTransformFieldsActionType.SET_DRAFT;
  payload: TransformEditableStep;
}

export interface SetResourceAction {
  type: PipelineTransformFieldsActionType.SET_RESOURCE;
  payload: Resource["name"];
}

export interface SaveAction {
  type: PipelineTransformFieldsActionType.SAVE;
}

export interface RemoveStepAction {
  type: PipelineTransformFieldsActionType.REMOVE_STEP;
  payload: TransformStep["id"];
}

export interface MoveStepAction {
  type: PipelineTransformFieldsActionType.MOVE_STEP;
  payload: { resource: Resource["name"]; from: number; to: number };
}

export type PipelineTransformFieldsAction =
  | LoadAction
  | OpenNewAction
  | OpenStepAction
  | CancelAction
  | SetDraftAction
  | SetResourceAction
  | SaveAction
  | RemoveStepAction
  | MoveStepAction;
