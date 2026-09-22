import type { JsonObject, JsonValue } from "@bufbuild/protobuf";
import type { DraggableAttributes, DraggableSyntheticListeners } from "@dnd-kit/core";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import type {
  ListTransformFunctionsResponse,
  TransformFunction,
} from "@/gen/ingestion/v1/transformations_pb";

export type TransformDefinition = JsonObject;

export enum TransformExprKind {
  EMPTY = "empty",
  COLUMN = "column",
  LITERAL = "literal",
  CALL = "call",
}

export enum TransformLiteralKind {
  STRING = "string",
  NUMBER = "number",
  BOOLEAN = "boolean",
}

export type TransformExpr =
  | { kind: TransformExprKind.EMPTY }
  | { kind: TransformExprKind.COLUMN; name: ResourceColumn["name"] }
  | { kind: TransformExprKind.LITERAL; literalKind: TransformLiteralKind; value: string | null }
  | { kind: TransformExprKind.CALL; fn: TransformFunction["name"]; args: TransformExpr[] };

export type TransformCallExpr = Extract<TransformExpr, { kind: TransformExprKind.CALL }>;
export type TransformLiteralExpr = Extract<TransformExpr, { kind: TransformExprKind.LITERAL }>;
export type TransformLeafExpr = Exclude<TransformExpr, TransformCallExpr>;

export interface TransformChainCall {
  fn: TransformFunction["name"];
  args: TransformExpr[];
}

export interface TransformChain {
  root: TransformLeafExpr;
  calls: TransformChainCall[];
}

export enum TransformConditionJoin {
  AND = "and",
  OR = "or",
}

export enum TransformStepKind {
  RENAME = "rename",
  DROP = "drop",
  COMPUTE = "compute",
  RAW = "raw",
}

export interface TransformRenamePair {
  from: ResourceColumn["name"];
  to: ResourceColumn["name"];
}

export interface TransformComputeOutput {
  name: ResourceColumn["name"];
  expr: TransformExpr;
}

export type TransformStep =
  | { id: string; kind: TransformStepKind.RENAME; pairs: TransformRenamePair[] }
  | { id: string; kind: TransformStepKind.DROP; names: ResourceColumn["name"][] }
  | {
      id: string;
      kind: TransformStepKind.COMPUTE;
      outputs: TransformComputeOutput[];
      where: TransformExpr | null;
    }
  | { id: string; kind: TransformStepKind.RAW; json: JsonValue };

export type TransformEditableStep = Exclude<TransformStep, { kind: TransformStepKind.RAW }>;
export type TransformComputeStep = Extract<TransformStep, { kind: TransformStepKind.COMPUTE }>;

export interface PipelineTransformFieldsDraft {
  resource: Resource["name"];
  id: TransformStep["id"] | null;
  step: TransformEditableStep;
}

export interface PipelineTransformFieldsState {
  stepsByResource: Map<Resource["name"], TransformStep[]>;
  draft: PipelineTransformFieldsDraft | null;
}

export interface PipelineTransformFieldsEnvironment {
  resources: Resource["name"][];
  columnsByResource: Map<Resource["name"], ResourceColumn[]>;
  sourceConnectionId: Connection["id"];
  functionsByName: Map<TransformFunction["name"], TransformFunction>;
  grammarVersion: ListTransformFunctionsResponse["grammarVersion"];
  isReadOnly: boolean;
}

export interface PipelineTransformFieldsEditor {
  columns: ResourceColumn[];
  types: Map<string, string>;
  errors: Map<string, string[]>;
  isDisabled: boolean;
}

export interface PipelineTransformFieldsStepHandle {
  ref: (element: HTMLElement | null) => void;
  attributes: DraggableAttributes;
  listeners: DraggableSyntheticListeners;
}
