import type { JsonValue } from "@bufbuild/protobuf";

import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";

export enum TransformStepKind {
  RENAME = "RENAME",
  DROP = "DROP",
  COMPUTE = "COMPUTE",
}

export enum TransformRowScope {
  ALL = "ALL",
  MATCHING = "MATCHING",
}

export type TransformLiteral = string | number | boolean;
export type TransformLiteralKind = "string" | "number" | "boolean";
export type TransformColumn = Pick<ResourceColumn, "name" | "logicalType" | "isPrimaryKey">;

/** The leaf an expression starts from before its function chain is applied. */
export type TransformExpressionSource =
  | { kind: "empty" }
  | { kind: "column"; column: ResourceColumn["name"] }
  | { kind: "literal"; literalKind: TransformLiteralKind; value: TransformLiteral | null };

/**
 * One function applied to the value flowing through an expression. `args`
 * contains every argument after that flowing value and may itself be nested.
 */
export interface TransformFunctionCall {
  name: string;
  args: TransformExpression[];
}

/** A leaf followed by zero or more calls. Later call arguments recurse. */
export interface TransformExpression {
  source: TransformExpressionSource;
  calls: TransformFunctionCall[];
}

export interface TransformRename {
  source: ResourceColumn["name"];
  target: ResourceColumn["name"];
}

export interface TransformComputeOutput {
  name: ResourceColumn["name"];
  expression: TransformExpression;
}

export interface TransformStepState {
  resource: Resource["name"];
  kind: TransformStepKind;
  renames: TransformRename[];
  drops: ResourceColumn["name"][];
  outputs: TransformComputeOutput[];
  rowScope: TransformRowScope;
  /** Kept as a draft when rowScope is ALL and serialized only when MATCHING. */
  where: TransformExpression;
}

/**
 * One step of a resource's definition as the builder shows it. A step the
 * builder cannot represent keeps its raw form and is shown read-only.
 */
export interface TransformStep extends TransformStepState {
  id: string;
  index: number;
  raw?: JsonValue;
}
