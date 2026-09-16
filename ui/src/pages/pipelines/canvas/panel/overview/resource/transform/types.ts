import type { JsonValue } from "@bufbuild/protobuf";

import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";

export enum TransformStepKind {
  RENAME = "RENAME",
  DROP = "DROP",
  COMPUTE = "COMPUTE",
}

/** One function applied to the value produced so far, with its optional literal argument. */
export interface TransformFunctionCall {
  name: string;
  literal: string;
}

export interface TransformStepState {
  resource: Resource["name"];
  column: ResourceColumn["name"];
  kind: TransformStepKind;
  rename: string;
  output: string;
  expression: TransformFunctionCall[];
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
