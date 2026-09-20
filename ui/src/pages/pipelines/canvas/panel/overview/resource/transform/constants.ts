import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import {
  TransformStepKind,
  type TransformStepState,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/types";

export const TRANSFORM_GRAMMAR_VERSION = 1;

export const TRANSFORM_STEP_KIND_TO_LABEL_MAP: Record<TransformStepKind, string> = {
  [TransformStepKind.RENAME]: "Rename",
  [TransformStepKind.DROP]: "Drop",
  [TransformStepKind.COMPUTE]: "Compute",
};

export const TRANSFORM_STEP_KIND_OPTIONS: SelectInputOption[] = Object.values(
  TransformStepKind,
).map((kind) => ({
  id: kind,
  label: TRANSFORM_STEP_KIND_TO_LABEL_MAP[kind],
  value: kind,
}));

export const TRANSFORM_STEP_DEFAULT_STATE: TransformStepState = {
  resource: "",
  column: "",
  kind: TransformStepKind.RENAME,
  rename: "",
  output: "",
  expression: [],
};
