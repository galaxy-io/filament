import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import type { TransformArgument, TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import {
  TRANSFORM_COPY_ACTION,
  TRANSFORM_FUNCTION_ORDER_MAP,
  TRANSFORM_INTEGER_TYPES,
  TRANSFORM_LITERAL_KIND_TO_ACTION_MAP,
  TRANSFORM_NUMBER_TYPES,
} from "@/components/transform/constants";
import {
  chainOf,
  createTransformColumnExpr,
  EMPTY_EXPR,
  rootColumnOf,
  trimTrailingEmptyExprs,
} from "@/components/transform/grammar/chain";
import {
  getTransformChainPaths,
  getTransformDropPath,
  getTransformOutputPath,
} from "@/components/transform/grammar/paths";
import {
  type PipelineTransformFieldsDraft,
  type PipelineTransformFieldsEditor,
  type TransformChainCall,
  type TransformComputeStep,
  type TransformEditableStep,
  type TransformExpr,
  TransformExprKind,
  type TransformLeafExpr,
  type TransformLiteralExpr,
  TransformLiteralKind,
  type TransformStep,
  TransformStepKind,
} from "@/components/transform/types";

const createTransformComputeStep = (
  column: string,
  id: string = crypto.randomUUID(),
): TransformComputeStep => ({
  id,
  kind: TransformStepKind.COMPUTE,
  outputs: [{ name: "", expr: createTransformColumnExpr(column) }],
  where: null,
});

export const createTransformStep = (
  kind: TransformEditableStep["kind"],
  column = "",
): TransformEditableStep => {
  switch (kind) {
    case TransformStepKind.RENAME:
      return { id: crypto.randomUUID(), kind, pairs: [{ from: column, to: "" }] };
    case TransformStepKind.DROP:
      return { id: crypto.randomUUID(), kind, names: [column] };
    case TransformStepKind.COMPUTE:
      return createTransformComputeStep(column);
  }
};

export const getTransformStepColumn = (step: TransformEditableStep): string => {
  switch (step.kind) {
    case TransformStepKind.RENAME:
      return step.pairs[0]?.from ?? "";
    case TransformStepKind.DROP:
      return step.names[0] ?? "";
    case TransformStepKind.COMPUTE:
      return rootColumnOf(step.outputs[0]?.expr ?? EMPTY_EXPR) ?? "";
  }
};

export const convertTransformStep = (
  step: TransformEditableStep,
  kind: TransformEditableStep["kind"],
): TransformEditableStep =>
  step.kind === kind
    ? step
    : { ...createTransformStep(kind, getTransformStepColumn(step)), id: step.id };

export const toTransformComputeStep = (step: TransformEditableStep): TransformComputeStep =>
  step.kind === TransformStepKind.COMPUTE
    ? step
    : createTransformComputeStep(getTransformStepColumn(step), step.id);

export const createTransformLiteral = (
  literalKind: TransformLiteralKind,
): TransformLiteralExpr => ({
  kind: TransformExprKind.LITERAL,
  literalKind,
  value: null,
});

export const getTransformExprAction = (
  expr: TransformExpr,
): { root: TransformLeafExpr; action: string } => {
  const chain = chainOf(expr);
  const action =
    chain.calls[0]?.fn ??
    (chain.root.kind === TransformExprKind.LITERAL
      ? TRANSFORM_LITERAL_KIND_TO_ACTION_MAP[chain.root.literalKind]
      : TRANSFORM_COPY_ACTION);
  return { root: chain.root, action };
};

export const getTransformStepAction = (
  step: TransformEditableStep,
): { root: TransformLeafExpr; action: string } =>
  step.kind === TransformStepKind.COMPUTE
    ? getTransformExprAction(step.outputs[0]?.expr ?? EMPTY_EXPR)
    : { root: createTransformColumnExpr(getTransformStepColumn(step)), action: step.kind };

export const placeTransformDraft = (
  stepsByResource: Map<Resource["name"], TransformStep[]>,
  draft: PipelineTransformFieldsDraft,
): { stepsByResource: Map<Resource["name"], TransformStep[]>; index: number } => {
  const next = new Map(stepsByResource);
  let index = -1;
  for (const [resource, steps] of next) {
    const found = steps.findIndex((step) => step.id === draft.id);
    if (found === -1) continue;
    if (resource === draft.resource) index = found;
    next.set(
      resource,
      steps.filter((step) => step.id !== draft.id),
    );
  }
  const target = next.get(draft.resource) ?? [];
  if (index === -1) index = target.length;
  next.set(draft.resource, [...target.slice(0, index), draft.step, ...target.slice(index)]);
  return { stepsByResource: next, index };
};

export const getTransformColumnType = (
  columns: ResourceColumn[],
  name: ResourceColumn["name"],
): string | undefined => columns.find((column) => column.name === name)?.logicalType;

export const getTransformExprType = (
  expr: TransformExpr,
  path: string,
  editor: PipelineTransformFieldsEditor,
): string | undefined =>
  expr.kind === TransformExprKind.COLUMN
    ? getTransformColumnType(editor.columns, expr.name)
    : editor.types.get(path);

export const getTransformArgumentSpec = (
  fn: TransformFunction,
  position: number,
): TransformArgument | undefined => {
  if (position < fn.args.length) return fn.args[position];
  const last = fn.args[fn.args.length - 1];
  return last?.isVariadic ? last : undefined;
};

export const getTransformArgumentTypes = (
  fn: TransformFunction,
  position: number,
  inputType: string | undefined,
): string[] => {
  const spec = getTransformArgumentSpec(fn, position);
  if (spec?.isLiteral) return spec.logicalTypes;
  if (fn.sameType && inputType !== undefined) return [inputType];
  return spec?.logicalTypes ?? [];
};

export const getTransformCallSlotCount = (fn: TransformFunction): number =>
  Math.max(0, fn.args.length - 1);

export const isTransformVariadic = (fn: TransformFunction): boolean =>
  fn.args[fn.args.length - 1]?.isVariadic === true;

export const getTransformLiteralKind = (
  logicalTypes: string[],
): TransformLiteralKind | undefined => {
  if (logicalTypes.length === 0) return undefined;
  if (logicalTypes.every((type) => type === "string")) return TransformLiteralKind.STRING;
  if (logicalTypes.every((type) => type === "bool")) return TransformLiteralKind.BOOLEAN;
  if (logicalTypes.every((type) => TRANSFORM_NUMBER_TYPES.has(type))) {
    return TransformLiteralKind.NUMBER;
  }
  return undefined;
};

export const isTransformIntegerOnly = (logicalTypes: string[]): boolean =>
  logicalTypes.length > 0 && logicalTypes.every((type) => TRANSFORM_INTEGER_TYPES.has(type));

export const getTransformNumberError = (value: string, isIntegerOnly: boolean): string | null => {
  const parsed = value.trim() === "" ? Number.NaN : Number(value);
  if (!Number.isFinite(parsed)) return "Enter a valid number.";
  if (isIntegerOnly && !Number.isSafeInteger(parsed)) return "Enter a whole number.";
  return null;
};

export const getTransformAcceptedColumns = (
  columns: ResourceColumn[],
  logicalTypes: string[],
): ResourceColumn[] =>
  columns.filter(
    (column) => logicalTypes.length === 0 || logicalTypes.includes(column.logicalType),
  );

export const createTransformCallArgs = (
  fn: TransformFunction,
  inputType: string | undefined,
  columns: ResourceColumn[],
  rootColumn: string | undefined,
): TransformExpr[] =>
  Array.from({ length: getTransformCallSlotCount(fn) }, (_, position) => {
    const spec = getTransformArgumentSpec(fn, position + 1);
    if (!spec || spec.isLiteral || spec.isOptional) return EMPTY_EXPR;
    const types = getTransformArgumentTypes(fn, position + 1, inputType);
    const literalKind = spec.isColumn ? undefined : getTransformLiteralKind(types);
    const accepted = getTransformAcceptedColumns(columns, types);
    if (literalKind !== undefined && literalKind !== TransformLiteralKind.BOOLEAN) {
      return accepted.length === 0 ? createTransformLiteral(literalKind) : EMPTY_EXPR;
    }
    const only = literalKind === undefined && accepted.length === 1 ? accepted[0] : undefined;
    return only !== undefined && only.name !== rootColumn
      ? createTransformColumnExpr(only.name)
      : EMPTY_EXPR;
  });

export const createTransformChainCall = (
  fn: TransformFunction["name"],
  current: TransformChainCall | undefined,
  inputType: string | undefined,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
  columns: ResourceColumn[],
  rootColumn: string | undefined,
): TransformChainCall => {
  const next = functionsByName.get(fn);
  if (!next) return { fn: "", args: [] };
  const keepsOperand = Boolean(
    current && functionsByName.get(current.fn)?.operatorSymbol && next.operatorSymbol,
  );
  return {
    fn,
    args:
      current && keepsOperand
        ? current.args
        : createTransformCallArgs(next, inputType, columns, rootColumn),
  };
};

export const replaceTransformChainCall = (
  calls: TransformChainCall[],
  index: number,
  fn: TransformFunction["name"],
  inputType: string | undefined,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
  columns: ResourceColumn[],
  rootColumn: string | undefined,
): TransformChainCall[] => {
  const call = createTransformChainCall(
    fn,
    calls[index],
    inputType,
    functionsByName,
    columns,
    rootColumn,
  );
  return index < calls.length
    ? calls.map((candidate, slot) => (slot === index ? call : candidate))
    : [...calls, call];
};

export const isTransformExprComplete = (
  expr: TransformExpr,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): boolean => {
  switch (expr.kind) {
    case TransformExprKind.EMPTY:
      return false;
    case TransformExprKind.COLUMN:
      return true;
    case TransformExprKind.LITERAL:
      return expr.value !== null;
    case TransformExprKind.CALL: {
      const fn = functionsByName.get(expr.fn);
      const [input = EMPTY_EXPR, ...rest] = expr.args;
      if (!fn || !isTransformExprComplete(input, functionsByName)) return false;
      const supplied = trimTrailingEmptyExprs(rest);
      const variadic = isTransformVariadic(fn);
      const fixed = fn.args.filter((arg) => !arg.isOptional && !arg.isVariadic).length;
      const required = variadic ? Math.max(fixed + 1, 2) : fixed;
      const total = supplied.length + 1;
      if (total < required || (!variadic && total > fn.args.length)) return false;
      return supplied.every(
        (arg, position) =>
          getTransformArgumentSpec(fn, position + 1) !== undefined &&
          isTransformExprComplete(arg, functionsByName),
      );
    }
  }
};

const compareTransformFunctions = (a: TransformFunction, b: TransformFunction): number =>
  (TRANSFORM_FUNCTION_ORDER_MAP.get(a.name) ?? TRANSFORM_FUNCTION_ORDER_MAP.size) -
    (TRANSFORM_FUNCTION_ORDER_MAP.get(b.name) ?? TRANSFORM_FUNCTION_ORDER_MAP.size) ||
  a.name.localeCompare(b.name);

export const getCompatibleTransformFunctions = (
  inputType: string | undefined,
  isLiteralInput: boolean,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): TransformFunction[] =>
  [...functionsByName.values()]
    .filter((fn) => {
      const first = fn.args[0];
      if (!first || inputType === undefined) return false;
      if (first.isColumn && isLiteralInput) return false;
      if (first.isLiteral && !isLiteralInput) return false;
      return first.logicalTypes.length === 0 || first.logicalTypes.includes(inputType);
    })
    .sort(compareTransformFunctions);

export const getTransformFunctionChoices = (
  input: TransformExpr,
  inputType: string | undefined,
  currentFn: TransformFunction["name"],
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): TransformFunction[] => {
  const compatible =
    input.kind === TransformExprKind.EMPTY
      ? [...functionsByName.values()]
          .filter((fn) => fn.args[0]?.isLiteral !== true)
          .sort(compareTransformFunctions)
      : getCompatibleTransformFunctions(
          inputType,
          input.kind === TransformExprKind.LITERAL,
          functionsByName,
        );
  const current = functionsByName.get(currentFn);
  return current && !compatible.some((fn) => fn.name === current.name)
    ? [...compatible, current]
    : compatible;
};

export const isTransformOutputNameIssue = (message: string): boolean =>
  message.startsWith("where ");

export const getTransformSubjectErrors = (
  step: TransformEditableStep,
  outputIndex: number,
  errors: PipelineTransformFieldsEditor["errors"],
): { isRootError: boolean; isActionError: boolean } => {
  if (step.kind === TransformStepKind.DROP) {
    return { isRootError: errors.has(getTransformDropPath(0)), isActionError: false };
  }
  if (step.kind === TransformStepKind.RENAME) return { isRootError: false, isActionError: false };
  const output = step.outputs[outputIndex];
  if (!output) return { isRootError: false, isActionError: false };
  const chain = chainOf(output.expr);
  const outputPath = getTransformOutputPath(output, step.where !== null);
  const paths = getTransformChainPaths(outputPath, chain);
  const outputMessages = errors.get(outputPath) ?? [];
  const isNameError = outputMessages.some(isTransformOutputNameIssue);
  return {
    isRootError: errors.has(paths.root),
    isActionError:
      chain.calls.length > 0
        ? errors.has(paths.calls[0])
        : !isNameError && outputMessages.length > 0,
  };
};

export const filterTransformOptions = (
  term: string,
  options: SelectInputOption[],
): SelectInputOption[] =>
  options.filter((option) => option.label.toLowerCase().includes(term.toLowerCase()));
