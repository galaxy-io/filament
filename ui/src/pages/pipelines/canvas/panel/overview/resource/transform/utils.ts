import type { JsonValue } from "@bufbuild/protobuf";

import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import { isJsonObject } from "@/components/fields/utils";

import { TRANSFORM_GRAMMAR_VERSION } from "@/pages/pipelines/canvas/panel/overview/resource/transform/constants";
import {
  type TransformFunctionCall,
  type TransformStep,
  TransformStepKind,
  type TransformStepState,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/types";
import type { PipelineCanvasEdgeTransform } from "@/pages/pipelines/canvas/types";

export const getTransformStepId = (resource: Resource["name"], index: number) =>
  `${resource}:${index}`;

const singleEntry = (value: JsonValue): [string, JsonValue] | null => {
  if (!isJsonObject(value)) return null;
  const entries = Object.entries(value);
  return entries.length === 1 ? entries[0] : null;
};

/**
 * Unwraps a nested call chain down to its column: `{year: {to_date: {col: x}}}`
 * becomes column `x` with steps `[to_date, year]`. Anything the builder cannot
 * show, such as a literal-only argument or a second column, returns null.
 */
const parseExpression = (
  expr: JsonValue,
): { column: ResourceColumn["name"]; expression: TransformFunctionCall[] } | null => {
  const entry = singleEntry(expr);
  if (!entry) return null;
  const [key, arg] = entry;
  if (key === "col") {
    return typeof arg === "string" ? { column: arg, expression: [] } : null;
  }
  const args = Array.isArray(arg) ? arg : [arg];
  if (args.length === 0 || args.length > 2) return null;
  const inner = parseExpression(args[0]);
  if (!inner) return null;
  const literal = args[1];
  if (literal !== undefined && typeof literal !== "string") return null;
  return {
    column: inner.column,
    expression: [...inner.expression, { name: key, literal: literal ?? "" }],
  };
};

const parseStep = (resource: Resource["name"], index: number, step: JsonValue): TransformStep => {
  const base = { id: getTransformStepId(resource, index), index, resource, rename: "", output: "" };
  const raw: TransformStep = {
    ...base,
    column: "",
    kind: TransformStepKind.COMPUTE,
    expression: [],
    raw: step,
  };
  if (!isJsonObject(step)) return raw;
  const entries = Object.entries(step);

  if (entries.length === 1 && entries[0][0] === "rename") {
    const rename = singleEntry(entries[0][1]);
    if (rename && typeof rename[1] === "string") {
      return {
        ...base,
        column: rename[0],
        kind: TransformStepKind.RENAME,
        rename: rename[1],
        expression: [],
      };
    }
  }
  if (entries.length === 1 && entries[0][0] === "drop") {
    const drop = entries[0][1];
    if (Array.isArray(drop) && drop.length === 1 && typeof drop[0] === "string") {
      return { ...base, column: drop[0], kind: TransformStepKind.DROP, expression: [] };
    }
  }
  if (entries.length === 1 && entries[0][0] === "compute") {
    const compute = singleEntry(entries[0][1]);
    const parsed = compute ? parseExpression(compute[1]) : null;
    if (compute && parsed) {
      return {
        ...base,
        column: parsed.column,
        kind: TransformStepKind.COMPUTE,
        output: compute[0] === parsed.column ? "" : compute[0],
        expression: parsed.expression,
      };
    }
  }
  return raw;
};

const getResourceSteps = (
  definition: PipelineCanvasEdgeTransform | undefined,
  resource: Resource["name"],
): JsonValue[] => {
  const resources = definition?.resources;
  if (resources === undefined || !isJsonObject(resources)) return [];
  const entry = resources[resource];
  if (entry === undefined || !isJsonObject(entry) || !Array.isArray(entry.steps)) return [];
  return entry.steps;
};

export const mapDefinitionToTransformSteps = (
  definition: PipelineCanvasEdgeTransform | undefined,
  resources: Resource["name"][],
): TransformStep[] =>
  resources.flatMap((resource) =>
    getResourceSteps(definition, resource).map((step, index) => parseStep(resource, index, step)),
  );

const buildExpression = (state: TransformStepState): JsonValue =>
  state.expression.reduce<JsonValue>(
    (inner, call) => ({ [call.name]: call.literal !== "" ? [inner, call.literal] : inner }),
    { col: state.column },
  );

const buildStep = (step: TransformStep): JsonValue => {
  if (step.raw !== undefined) return step.raw;
  switch (step.kind) {
    case TransformStepKind.RENAME:
      return { rename: { [step.column]: step.rename } };
    case TransformStepKind.DROP:
      return { drop: [step.column] };
    case TransformStepKind.COMPUTE:
      return { compute: { [step.output || step.column]: buildExpression(step) } };
  }
};

/** Renders steps back into a definition, or undefined when there are none left. */
export const mapTransformStepsToDefinition = (
  steps: TransformStep[],
): PipelineCanvasEdgeTransform | undefined => {
  const resources: Record<string, JsonValue> = {};
  for (const step of steps) {
    const entry = resources[step.resource];
    const steps =
      entry !== undefined && isJsonObject(entry) && Array.isArray(entry.steps) ? entry.steps : [];
    resources[step.resource] = { steps: [...steps, buildStep(step)] };
  }
  if (Object.keys(resources).length === 0) return undefined;
  return { version: TRANSFORM_GRAMMAR_VERSION, resources };
};

export const isTransformStepValid = (state: TransformStepState): boolean => {
  if (state.resource === "" || state.column === "") return false;
  switch (state.kind) {
    case TransformStepKind.RENAME:
      return state.rename.trim() !== "" && state.rename.trim() !== state.column;
    case TransformStepKind.DROP:
      return true;
    case TransformStepKind.COMPUTE:
      return state.expression.length > 0 && state.expression.every((call) => call.name !== "");
  }
};

/** The logical type after every function in the chain, or "" when one is unknown. */
export const getTransformExpressionType = (
  columnType: ResourceColumn["logicalType"],
  expression: TransformFunctionCall[],
  functionsByName: Map<string, TransformFunction>,
): ResourceColumn["logicalType"] =>
  expression.reduce(
    (type, call) => (type === "" ? "" : (functionsByName.get(call.name)?.returns ?? "")),
    columnType,
  );

/** The step as one line, in the shape a reader of the yaml would recognise. */
export const formatTransformStepSummary = (step: TransformStep): string => {
  if (step.raw !== undefined) return "Defined outside the builder";
  switch (step.kind) {
    case TransformStepKind.RENAME:
      return `${step.column} → ${step.rename}`;
    case TransformStepKind.DROP:
      return `drop ${step.column}`;
    case TransformStepKind.COMPUTE: {
      const value = step.expression.reduce((inner, call) => `${call.name}(${inner})`, step.column);
      return `${step.output || step.column} = ${value}`;
    }
  }
};

const ISSUE_PATH_PATTERN = /^resources\["([^"]+)"\]\.steps\[(\d+)\]/;

/** Maps a validation issue path to the step it sits on, when it names one. */
export const getTransformIssueStepId = (path: string): string | null => {
  const match = ISSUE_PATH_PATTERN.exec(path);
  return match ? getTransformStepId(match[1], Number(match[2])) : null;
};
