import type { JsonValue } from "@bufbuild/protobuf";

import type { Resource } from "@/gen/ingestion/v1/connectors_pb";
import type {
  TransformArgument as TransformArgumentSpec,
  TransformFunction,
} from "@/gen/ingestion/v1/transformations_pb";

import { isJsonObject } from "@/components/fields/utils";

import {
  createEmptyTransformExpression,
  createTransformStepDefaultState,
  TRANSFORM_GRAMMAR_VERSION,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/constants";
import {
  type TransformColumn,
  type TransformComputeOutput,
  type TransformExpression,
  type TransformExpressionSource,
  type TransformFunctionCall,
  type TransformLiteral,
  type TransformLiteralKind,
  TransformRowScope,
  type TransformStep,
  TransformStepKind,
  type TransformStepState,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/types";
import type { PipelineCanvasEdgeTransform } from "@/pages/pipelines/canvas/types";

const INTEGER_TYPES = new Set(["int16", "int32", "int64"]);
const NUMBER_TYPES = new Set(["int16", "int32", "int64", "float32", "float64"]);

export const getTransformStepId = (resource: Resource["name"], index: number) =>
  `${resource}:${index}`;

const singleEntry = (value: JsonValue): [string, JsonValue] | null => {
  if (!isJsonObject(value)) return null;
  const entries = Object.entries(value);
  return entries.length === 1 ? entries[0] : null;
};

const literalSource = (value: TransformLiteral): TransformExpressionSource => ({
  kind: "literal",
  literalKind: typeof value as TransformLiteralKind,
  value,
});

/**
 * Every expression can be represented as a leaf plus a chain along the first
 * argument. Arguments after the first recurse into the same representation.
 */
const parseExpression = (
  expr: JsonValue,
  allowNestedArguments = false,
): TransformExpression | null => {
  if (typeof expr === "string" || typeof expr === "number" || typeof expr === "boolean") {
    return { source: literalSource(expr), calls: [] };
  }

  const entry = singleEntry(expr);
  if (!entry) return null;
  const [key, arg] = entry;
  if (key === "col") {
    return typeof arg === "string" && arg !== ""
      ? { source: { kind: "column", column: arg }, calls: [] }
      : null;
  }

  const args = Array.isArray(arg) ? arg : [arg];
  if (args.length === 0) return null;
  const inner = parseExpression(args[0], allowNestedArguments);
  const rest = args.slice(1).map((value) => parseExpression(value, allowNestedArguments));
  if (!inner || rest.some((candidate) => candidate === null)) return null;
  if (
    !allowNestedArguments &&
    rest.some((candidate) => candidate !== null && candidate.calls.length > 0)
  ) {
    return null;
  }
  return {
    ...inner,
    calls: [...inner.calls, { name: key, args: rest as TransformExpression[] }],
  };
};

const parseStep = (resource: Resource["name"], index: number, step: JsonValue): TransformStep => {
  const base = {
    ...createTransformStepDefaultState(resource),
    id: getTransformStepId(resource, index),
    index,
  };
  const raw: TransformStep = { ...base, raw: step };
  if (!isJsonObject(step)) return raw;
  const entries = Object.entries(step);

  if (entries.length === 1 && entries[0][0] === "rename" && isJsonObject(entries[0][1])) {
    const renames = Object.entries(entries[0][1]);
    if (
      renames.length === 1 &&
      renames.every(
        ([source, target]) => source !== "" && typeof target === "string" && target !== "",
      )
    ) {
      return {
        ...base,
        kind: TransformStepKind.RENAME,
        renames: renames.map(([source, target]) => ({ source, target: target as string })),
      };
    }
  }

  if (entries.length === 1 && entries[0][0] === "drop") {
    const drops = entries[0][1];
    if (
      Array.isArray(drops) &&
      drops.length === 1 &&
      drops.every((name): name is string => typeof name === "string" && name !== "")
    ) {
      return { ...base, kind: TransformStepKind.DROP, drops };
    }
  }

  const keys = entries.map(([key]) => key);
  if (
    keys.includes("compute") &&
    keys.every((key) => key === "compute" || key === "where") &&
    isJsonObject(step.compute)
  ) {
    const outputs = Object.entries(step.compute).map(([name, expr]) => {
      if (name === "") return null;
      const expression = parseExpression(expr);
      return expression ? { name, expression } : null;
    });
    const where =
      step.where === undefined
        ? createEmptyTransformExpression()
        : parseExpression(step.where, true);
    if (outputs.length > 0 && outputs.every((output) => output !== null) && where) {
      return {
        ...base,
        kind: TransformStepKind.COMPUTE,
        outputs: (outputs as TransformComputeOutput[]).map((output) => ({
          ...output,
          name:
            output.expression.source.kind === "column" &&
            output.name === output.expression.source.column
              ? ""
              : output.name,
        })),
        rowScope: step.where === undefined ? TransformRowScope.ALL : TransformRowScope.MATCHING,
        where,
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

export const isTransformExpressionEmpty = (expression: TransformExpression): boolean =>
  expression.source.kind === "empty" && expression.calls.length === 0;

const buildSource = (source: TransformExpressionSource): JsonValue => {
  switch (source.kind) {
    case "column":
      return { col: source.column };
    case "literal":
      return source.value;
    case "empty":
      return null;
  }
};

/** Empty optional arguments at the end are omitted without collapsing positions in the middle. */
const buildExpression = (expression: TransformExpression): JsonValue =>
  expression.calls.reduce<JsonValue>((inner, call) => {
    let last = call.args.length - 1;
    while (last >= 0 && isTransformExpressionEmpty(call.args[last])) last--;
    const args = call.args.slice(0, last + 1).map(buildExpression);
    return { [call.name]: args.length > 0 ? [inner, ...args] : inner };
  }, buildSource(expression.source));

export const getTransformOutputName = (output: TransformComputeOutput): string => {
  const explicit = output.name.trim();
  if (explicit !== "") return explicit;
  return output.expression.source.kind === "column" ? output.expression.source.column : "";
};

const buildStep = (step: TransformStep): JsonValue => {
  if (step.raw !== undefined) return step.raw;
  switch (step.kind) {
    case TransformStepKind.RENAME:
      return {
        rename: Object.fromEntries(
          step.renames.map((rename) => [rename.source, rename.target.trim()]),
        ),
      };
    case TransformStepKind.DROP:
      return { drop: step.drops };
    case TransformStepKind.COMPUTE: {
      const compute = Object.fromEntries(
        step.outputs.map((output) => [
          getTransformOutputName(output),
          buildExpression(output.expression),
        ]),
      );
      return step.rowScope === TransformRowScope.MATCHING
        ? { compute, where: buildExpression(step.where) }
        : { compute };
    }
  }
};

/** Renders steps back into a definition, or undefined when there are none left. */
export const mapTransformStepsToDefinition = (
  steps: TransformStep[],
): PipelineCanvasEdgeTransform | undefined => {
  const resources: Record<string, JsonValue> = {};
  for (const step of steps) {
    const entry = resources[step.resource];
    const resourceSteps =
      entry !== undefined && isJsonObject(entry) && Array.isArray(entry.steps) ? entry.steps : [];
    resources[step.resource] = { steps: [...resourceSteps, buildStep(step)] };
  }
  if (Object.keys(resources).length === 0) return undefined;
  return { version: TRANSFORM_GRAMMAR_VERSION, resources };
};

/** The catalog's spec for argument position `index`; only a variadic tail repeats. */
export const getTransformArgumentSpec = (
  fn: TransformFunction,
  index: number,
): TransformArgumentSpec | undefined => {
  if (index < fn.args.length) return fn.args[index];
  const last = fn.args[fn.args.length - 1];
  return last?.isVariadic ? last : undefined;
};

export const getTransformLiteralKind = (
  logicalTypes: string[],
): TransformLiteralKind | undefined => {
  if (logicalTypes.length === 0) return undefined;
  if (logicalTypes.every((type) => type === "string")) return "string";
  if (logicalTypes.every((type) => type === "bool")) return "boolean";
  if (logicalTypes.every((type) => NUMBER_TYPES.has(type))) return "number";
  return undefined;
};

export const getTransformLiteralKindForType = (
  logicalType: string,
): TransformLiteralKind | undefined => {
  if (logicalType === "string") return "string";
  if (logicalType === "bool") return "boolean";
  if (NUMBER_TYPES.has(logicalType)) return "number";
  return undefined;
};

export interface TransformExpressionInfo {
  type: string;
  isLiteral: boolean;
  complete: boolean;
  error?: string;
  literalValue?: TransformLiteral;
  /** Display name of the function that produced this result, when applicable. */
  producerName?: string;
}

const incomplete = (error: string): TransformExpressionInfo => ({
  type: "",
  isLiteral: false,
  complete: false,
  error,
});

const sourceInfo = (
  source: TransformExpressionSource,
  columns: TransformColumn[],
): TransformExpressionInfo => {
  switch (source.kind) {
    case "empty":
      return incomplete("Choose a column or value.");
    case "column": {
      const column = columns.find((candidate) => candidate.name === source.column);
      return column
        ? { type: column.logicalType, isLiteral: false, complete: true }
        : incomplete(`Column ${source.column || "…"} does not exist at this step.`);
    }
    case "literal": {
      if (source.value === null) return incomplete("Enter a value.");
      if (typeof source.value !== source.literalKind) return incomplete("Enter a valid value.");
      if (typeof source.value === "number") {
        if (!Number.isFinite(source.value)) return incomplete("Enter a finite number.");
        if (Number.isInteger(source.value) && !Number.isSafeInteger(source.value)) {
          return incomplete("Enter an integer within JavaScript's safe range.");
        }
      }
      return {
        type:
          typeof source.value === "string"
            ? "string"
            : typeof source.value === "boolean"
              ? "bool"
              : Number.isInteger(source.value)
                ? "int64"
                : "float64",
        isLiteral: true,
        complete: true,
        literalValue: source.value,
      };
    }
  }
};

const coercibleLiteral = (info: TransformExpressionInfo, target: string): boolean => {
  if (!info.isLiteral || typeof info.literalValue !== "number") return false;
  const value = info.literalValue;
  switch (target) {
    case "int16":
      return Number.isInteger(value) && value >= -(2 ** 15) && value < 2 ** 15;
    case "int32":
      return Number.isInteger(value) && value >= -(2 ** 31) && value < 2 ** 31;
    case "int64":
      return Number.isSafeInteger(value);
    case "float32":
    case "float64":
      return true;
    default:
      return false;
  }
};

const logicalTypeNoun = (type: string): string => {
  if (NUMBER_TYPES.has(type) || type === "decimal") return "number";
  switch (type) {
    case "bool":
      return "boolean";
    case "string":
      return "text";
    case "bytes":
      return "bytes";
    case "date":
      return "date";
    case "time":
      return "time";
    case "timestamp":
    case "timestamptz":
      return "timestamp";
    case "json":
      return "JSON value";
    case "uuid":
      return "UUID";
    case "array":
      return "array";
    default:
      return type;
  }
};

const logicalTypePhrase = (type: string): string => {
  const noun = logicalTypeNoun(type);
  if (noun === "text" || noun === "bytes") return noun;
  return `${noun === "array" ? "an" : "a"} ${noun}`;
};

const acceptedTypesPhrase = (types: string[]): string => {
  const nouns = [...new Set(types.map(logicalTypeNoun))];
  if (nouns.length === 0) return "a supported value";
  if (nouns.length === 1) return logicalTypePhrase(types[0]);
  if (nouns.length === 2) return `a ${nouns[0]} or ${nouns[1]}`;
  return `one of ${nouns.slice(0, -1).join(", ")}, or ${nouns[nouns.length - 1]}`;
};

const callInfo = (
  input: TransformExpressionInfo,
  call: TransformFunctionCall,
  columns: TransformColumn[],
  functionsByName: Map<string, TransformFunction>,
): TransformExpressionInfo => {
  if (!input.complete) return input;
  const fn = functionsByName.get(call.name);
  if (!fn)
    return incomplete(call.name === "" ? "Choose a function." : `Unknown function ${call.name}.`);
  const functionName = fn.displayName || fn.name;

  let last = call.args.length - 1;
  while (last >= 0 && isTransformExpressionEmpty(call.args[last])) last--;
  const supplied = call.args.slice(0, last + 1);
  const variadic = fn.args[fn.args.length - 1]?.isVariadic === true;
  const required =
    fn.args.filter((arg) => !arg.isOptional && !arg.isVariadic).length + (variadic ? 1 : 0);
  const total = supplied.length + 1;
  if (total < required) return incomplete(`${functionName} needs another argument.`);
  if (!variadic && total > fn.args.length) {
    return incomplete(`${functionName} has too many arguments.`);
  }

  const infos = [
    input,
    ...supplied.map((arg) => getTransformExpressionInfo(arg, columns, functionsByName)),
  ];
  for (let index = 0; index < infos.length; index++) {
    const info = infos[index];
    const spec = getTransformArgumentSpec(fn, index);
    if (!spec) return incomplete(`${functionName} has too many arguments.`);
    if (!info.complete)
      return incomplete(
        `${functionName}'s ${spec.name}: ${info.error ?? "Complete this argument."}`,
      );
  }

  const shared = fn.sameType
    ? (infos.find(
        (info, index) => !info.isLiteral && !getTransformArgumentSpec(fn, index)?.isLiteral,
      )?.type ?? "")
    : "";
  if (fn.sameType && shared === "") {
    return incomplete(`${functionName} needs at least one column.`);
  }

  for (let index = 0; index < infos.length; index++) {
    const info = infos[index];
    const spec = getTransformArgumentSpec(fn, index);
    if (!spec) continue;
    if (spec.isColumn && info.isLiteral) {
      return incomplete(`${functionName}'s ${spec.name} must be a column.`);
    }
    if (spec.isLiteral && !info.isLiteral) {
      return incomplete(`${functionName}'s ${spec.name} must be a value.`);
    }

    const effectiveType =
      fn.sameType && !spec.isLiteral && info.type !== shared && coercibleLiteral(info, shared)
        ? shared
        : info.type;
    if (spec.logicalTypes.length > 0 && !spec.logicalTypes.includes(effectiveType)) {
      if (index === 0) {
        const producer = input.producerName ?? "The input";
        return incomplete(
          `${functionName} needs ${acceptedTypesPhrase(spec.logicalTypes)}; ${producer} returns ${logicalTypePhrase(info.type)}.`,
        );
      }
      return incomplete(
        `${functionName}'s ${spec.name} needs ${acceptedTypesPhrase(spec.logicalTypes)}; the selected value is ${logicalTypePhrase(info.type)}.`,
      );
    }
    if (fn.sameType && !spec.isLiteral && effectiveType !== shared) {
      return incomplete(
        `${functionName} needs matching types; ${spec.name} is ${logicalTypePhrase(info.type)}, while the left input is ${logicalTypePhrase(shared)}.`,
      );
    }
  }

  const type = fn.returnsInput ? shared || input.type : fn.returns;
  return type === ""
    ? incomplete(`${functionName} has no result type.`)
    : { type, isLiteral: false, complete: true, producerName: functionName };
};

export const getTransformExpressionInfo = (
  expression: TransformExpression,
  columns: TransformColumn[],
  functionsByName: Map<string, TransformFunction>,
): TransformExpressionInfo =>
  expression.calls.reduce(
    (info, call) => callInfo(info, call, columns, functionsByName),
    sourceInfo(expression.source, columns),
  );

export const getTransformExpressionType = (
  expression: TransformExpression,
  columns: TransformColumn[],
  functionsByName: Map<string, TransformFunction>,
): string => getTransformExpressionInfo(expression, columns, functionsByName).type;

export const getCompatibleTransformFunctions = (
  input: TransformExpressionInfo,
  functionsByName: Map<string, TransformFunction>,
): TransformFunction[] => {
  if (!input.complete) return [];
  return [...functionsByName.values()].filter((fn) => {
    const first = fn.args[0];
    if (!first) return false;
    if (first.isColumn && input.isLiteral) return false;
    if (first.isLiteral && !input.isLiteral) return false;
    return first.logicalTypes.length === 0 || first.logicalTypes.includes(input.type);
  });
};

/**
 * Condition-role fields were added after the original catalog endpoint. Keep
 * the builder stable while an older server response is still cached during a
 * rolling deploy, but prefer the explicit backend roles as soon as they exist.
 */
const hasConditionRoleMetadata = (functionsByName: Map<string, TransformFunction>): boolean =>
  [...functionsByName.values()].some(
    (fn) => fn.conditionOperator || fn.conditionJoin === "and" || fn.conditionJoin === "or",
  );

export const isTransformConditionOperator = (
  fn: TransformFunction | undefined,
  functionsByName: Map<string, TransformFunction>,
): boolean => {
  if (!fn) return false;
  if (hasConditionRoleMetadata(functionsByName)) return fn.conditionOperator;
  return fn.returns === "bool" && fn.name !== "and" && fn.name !== "or" && fn.name !== "not";
};

export const getTransformConditionJoin = (
  fn: TransformFunction | undefined,
  functionsByName: Map<string, TransformFunction>,
): "and" | "or" | undefined => {
  if (!fn) return undefined;
  if (fn.conditionJoin === "and" || fn.conditionJoin === "or") return fn.conditionJoin;
  if (hasConditionRoleMetadata(functionsByName)) return undefined;
  return fn.name === "and" || fn.name === "or" ? fn.name : undefined;
};

export const getTransformStepValidationError = (
  state: TransformStepState,
  columns: TransformColumn[],
  functionsByName: Map<string, TransformFunction>,
): string | null => {
  if (state.resource === "") return "Choose a resource.";
  const typeOf = (name: string) =>
    columns.find((column) => column.name === name)?.logicalType ?? "";

  switch (state.kind) {
    case TransformStepKind.RENAME: {
      if (state.renames.length === 0) return "Add a column to rename.";
      const sources = new Set<string>();
      const targets = new Set<string>();
      for (const rename of state.renames) {
        if (typeOf(rename.source) === "") return "Choose an existing column to rename.";
        if (rename.target.trim() === "") return "Enter a new column name.";
        if (rename.target.trim() === rename.source) return "The new column name must be different.";
        if (sources.has(rename.source)) return `Column ${rename.source} is renamed more than once.`;
        if (targets.has(rename.target.trim()))
          return `Column ${rename.target.trim()} is used more than once.`;
        sources.add(rename.source);
        targets.add(rename.target.trim());
      }
      const names = new Set(columns.map((column) => column.name));
      for (const rename of [...state.renames].sort((left, right) =>
        left.source.localeCompare(right.source),
      )) {
        const target = rename.target.trim();
        if (names.has(target)) return `Column ${target} already exists.`;
        names.delete(rename.source);
        names.add(target);
      }
      return null;
    }
    case TransformStepKind.DROP: {
      if (state.drops.length === 0) return "Add a column to drop.";
      const names = new Set<string>();
      for (const name of state.drops) {
        const column = columns.find((candidate) => candidate.name === name);
        if (!column) return "Choose an existing column to drop.";
        if (column.isPrimaryKey) return `Primary key column ${name} cannot be dropped.`;
        if (names.has(name)) return `Column ${name} is selected more than once.`;
        names.add(name);
      }
      return null;
    }
    case TransformStepKind.COMPUTE: {
      if (state.outputs.length === 0) return "Add an output column.";
      const names = new Set<string>();
      for (const output of state.outputs) {
        const info = getTransformExpressionInfo(output.expression, columns, functionsByName);
        if (!info.complete) return info.error ?? "Complete the expression.";
        const name = getTransformOutputName(output);
        if (name === "") return "Enter an output column name.";
        if (names.has(name)) return `Output column ${name} is used more than once.`;
        names.add(name);
        if (state.rowScope === TransformRowScope.MATCHING) {
          const targetType = typeOf(name);
          if (targetType === "") return `Matching rows requires ${name} to already exist.`;
          if (targetType !== info.type) {
            return `Matching rows cannot change ${name} from ${targetType} to ${info.type}.`;
          }
        }
      }
      if (state.rowScope === TransformRowScope.MATCHING) {
        const condition = getTransformExpressionInfo(state.where, columns, functionsByName);
        if (!condition.complete) return condition.error ?? "Complete the condition.";
        if (condition.type !== "bool")
          return `The condition must return bool, not ${condition.type}.`;
      }
      return null;
    }
  }
};

export const isTransformStepValid = (
  state: TransformStepState,
  columns: TransformColumn[],
  functionsByName: Map<string, TransformFunction>,
): boolean => getTransformStepValidationError(state, columns, functionsByName) === null;

const formatExpression = (expression: TransformExpression): string => {
  let value: string;
  switch (expression.source.kind) {
    case "column":
      value = expression.source.column;
      break;
    case "literal":
      value = expression.source.value === null ? "…" : JSON.stringify(expression.source.value);
      break;
    case "empty":
      value = "…";
      break;
  }
  return expression.calls.reduce(
    (inner, call) =>
      `${call.name}(${[inner, ...call.args.filter((arg) => !isTransformExpressionEmpty(arg)).map(formatExpression)].join(", ")})`,
    value,
  );
};

/** The step as one line, in the shape a reader of the yaml would recognise. */
export const formatTransformStepSummary = (step: TransformStep): string => {
  if (step.raw !== undefined) return "Defined outside the builder";
  switch (step.kind) {
    case TransformStepKind.RENAME:
      return step.renames.map((rename) => `${rename.source} → ${rename.target}`).join(", ");
    case TransformStepKind.DROP:
      return `drop ${step.drops.join(", ")}`;
    case TransformStepKind.COMPUTE: {
      const outputs = step.outputs
        .map(
          (output) => `${getTransformOutputName(output)} = ${formatExpression(output.expression)}`,
        )
        .join("; ");
      const where =
        step.rowScope === TransformRowScope.MATCHING
          ? ` where ${formatExpression(step.where)}`
          : "";
      return `${outputs}${where}`;
    }
  }
};

const ISSUE_PATH_PATTERN = /^resources\["([^"]+)"\]\.steps\[(\d+)\]/;

/** Maps a validation issue path to the step it sits on, when it names one. */
export const getTransformIssueStepId = (path: string): string | null => {
  const match = ISSUE_PATH_PATTERN.exec(path);
  return match ? getTransformStepId(match[1], Number(match[2])) : null;
};

export const isIntegerLogicalType = (type: string): boolean => INTEGER_TYPES.has(type);
