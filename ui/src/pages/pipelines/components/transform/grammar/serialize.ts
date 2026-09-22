import type { JsonValue } from "@bufbuild/protobuf";

import type { Resource } from "@/gen/ingestion/v1/connectors_pb";

import { TRANSFORM_NEW_COLUMN_SUFFIX } from "@/pages/pipelines/components/transform/constants";
import {
  getTransformRootColumn,
  TRANSFORM_EMPTY_EXPR,
  trimTrailingEmptyExprs,
} from "@/pages/pipelines/components/transform/grammar/chain";
import {
  type TransformComputeOutput,
  type TransformDefinition,
  type TransformExpr,
  TransformExprKind,
  type TransformLiteralExpr,
  TransformLiteralKind,
  type TransformStep,
  TransformStepKind,
} from "@/pages/pipelines/components/transform/types";

export const getTransformOutputName = (
  output: TransformComputeOutput,
  isMatchingRows: boolean,
): string => {
  const explicit = output.name.trim();
  if (explicit !== "") return explicit;
  const root = getTransformRootColumn(output.expr);
  if (root === undefined) return "";
  return isMatchingRows ? root : `${root}${TRANSFORM_NEW_COLUMN_SUFFIX}`;
};

export const isTransformOutputInPlace = (
  output: TransformComputeOutput,
  isMatchingRows: boolean,
): boolean =>
  getTransformOutputName(output, isMatchingRows) === getTransformRootColumn(output.expr);

const serializeTransformLiteral = ({ literalKind, value }: TransformLiteralExpr): JsonValue => {
  if (value === null) return null;
  switch (literalKind) {
    case TransformLiteralKind.STRING:
      return value;
    case TransformLiteralKind.NUMBER:
      return Number(value);
    case TransformLiteralKind.BOOLEAN:
      return value === "true";
  }
};

export const serializeTransformExpr = (expr: TransformExpr): JsonValue => {
  switch (expr.kind) {
    case TransformExprKind.EMPTY:
      return null;
    case TransformExprKind.COLUMN:
      return { col: expr.name };
    case TransformExprKind.LITERAL:
      return serializeTransformLiteral(expr);
    case TransformExprKind.CALL: {
      if (expr.fn === "") return serializeTransformExpr(expr.args[0] ?? TRANSFORM_EMPTY_EXPR);
      const args = trimTrailingEmptyExprs(expr.args).map(serializeTransformExpr);
      return { [expr.fn]: args.length === 1 ? args[0] : args };
    }
  }
};

export const serializeTransformStep = (step: TransformStep): JsonValue => {
  switch (step.kind) {
    case TransformStepKind.RAW:
      return step.json;
    case TransformStepKind.RENAME:
      return { rename: Object.fromEntries(step.pairs.map((pair) => [pair.from, pair.to.trim()])) };
    case TransformStepKind.DROP:
      return { drop: [...step.names] };
    case TransformStepKind.COMPUTE: {
      const compute = Object.fromEntries(
        step.outputs.map((output) => [
          getTransformOutputName(output, step.where !== null),
          serializeTransformExpr(output.expr),
        ]),
      );
      return step.where ? { compute, where: serializeTransformExpr(step.where) } : { compute };
    }
  }
};

export const serializeTransformDefinition = (
  stepsByResource: Map<Resource["name"], TransformStep[]>,
  grammarVersion: number,
): TransformDefinition | undefined => {
  const resources: Record<string, JsonValue> = {};
  for (const [resource, steps] of stepsByResource) {
    if (steps.length === 0) continue;
    resources[resource] = { steps: steps.map(serializeTransformStep) };
  }
  if (Object.keys(resources).length === 0) return undefined;
  return { version: grammarVersion, resources };
};
