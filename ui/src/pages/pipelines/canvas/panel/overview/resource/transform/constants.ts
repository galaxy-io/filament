import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import {
  type TransformExpression,
  TransformRowScope,
  TransformStepKind,
  type TransformStepState,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/types";

export const TRANSFORM_GRAMMAR_VERSION = 1;
export const TRANSFORM_FLOW_SOURCE_WIDTH = 132;
export const TRANSFORM_INLINE_BINARY_FUNCTION_WIDTH = 64;

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

/** Display-only symbols for the comparison family; wire names remain unchanged. */
export const TRANSFORM_COMPARISON_SYMBOLS: ReadonlyMap<string, string> = new Map([
  ["eq", "="],
  ["neq", "≠"],
  ["gt", ">"],
  ["gte", "≥"],
  ["lt", "<"],
  ["lte", "≤"],
]);

/** Display-only symbols for arithmetic functions using the shared binary row. */
export const TRANSFORM_ARITHMETIC_SYMBOLS: ReadonlyMap<string, string> = new Map([
  ["add", "+"],
  ["sub", "−"],
  ["mul", "×"],
  ["div", "÷"],
]);

export const TRANSFORM_INLINE_BINARY_SYMBOLS: ReadonlyMap<string, string> = new Map([
  ...TRANSFORM_COMPARISON_SYMBOLS,
  ...TRANSFORM_ARITHMETIC_SYMBOLS,
]);

export const createEmptyTransformExpression = (): TransformExpression => ({
  source: { kind: "empty" },
  calls: [],
});

export const createTransformStepDefaultState = (resource = ""): TransformStepState => ({
  resource,
  kind: TransformStepKind.RENAME,
  renames: [{ source: "", target: "" }],
  drops: [""],
  outputs: [{ name: "", expression: createEmptyTransformExpression() }],
  rowScope: TransformRowScope.ALL,
  where: createEmptyTransformExpression(),
});
