import { match } from "ts-pattern";

import type { ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import type { TransformArgument, TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import {
  TRANSFORM_INTEGER_TYPES,
  TRANSFORM_NUMBER_TYPES,
} from "@/pages/pipelines/components/transform/constants";
import {
  TRANSFORM_EMPTY_EXPR,
  trimTrailingEmptyExprs,
} from "@/pages/pipelines/components/transform/grammar/chain";
import {
  type TransformExpr,
  TransformExprKind,
  TransformLiteralKind,
} from "@/pages/pipelines/components/transform/types";

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

export const isTransformVariadic = (fn: TransformFunction): boolean =>
  fn.args[fn.args.length - 1]?.isVariadic === true;

export const getTransformCallSlotCount = (fn: TransformFunction): number =>
  Math.max(fn.args.length - 1, isTransformVariadic(fn) ? 1 : 0);

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

export const getTransformAcceptedColumns = (
  columns: ResourceColumn[],
  logicalTypes: string[],
): ResourceColumn[] =>
  columns.filter(
    (column) => logicalTypes.length === 0 || logicalTypes.includes(column.logicalType),
  );

export const isTransformExprComplete = (
  expr: TransformExpr,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): boolean =>
  match(expr)
    .with({ kind: TransformExprKind.EMPTY }, () => false)
    .with({ kind: TransformExprKind.COLUMN }, () => true)
    .with({ kind: TransformExprKind.LITERAL }, ({ value }) => value !== null)
    .with({ kind: TransformExprKind.CALL }, (call) => {
      const fn = functionsByName.get(call.fn);
      const [input = TRANSFORM_EMPTY_EXPR, ...rest] = call.args;
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
    })
    .exhaustive();

export const getCompatibleTransformFunctions = (
  inputType: string | undefined,
  isLiteralInput: boolean,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): TransformFunction[] =>
  [...functionsByName.values()].filter((fn) => {
    const first = fn.args[0];
    if (!first || inputType === undefined) return false;
    if (first.isColumn && isLiteralInput) return false;
    if (first.isLiteral && !isLiteralInput) return false;
    return first.logicalTypes.length === 0 || first.logicalTypes.includes(inputType);
  });

export const getTransformFunctionChoices = (
  input: TransformExpr,
  inputType: string | undefined,
  currentFn: TransformFunction["name"],
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): TransformFunction[] => {
  const compatible =
    input.kind === TransformExprKind.EMPTY
      ? [...functionsByName.values()].filter((fn) => fn.args[0]?.isLiteral !== true)
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
