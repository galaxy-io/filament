import type { JsonValue } from "@bufbuild/protobuf";
import { match } from "ts-pattern";

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

const serializeTransformLiteral = ({ literalKind, value }: TransformLiteralExpr): JsonValue =>
  value === null
    ? null
    : match<TransformLiteralKind, JsonValue>(literalKind)
        .with(TransformLiteralKind.STRING, () => value)
        .with(TransformLiteralKind.NUMBER, () => Number(value))
        .with(TransformLiteralKind.BOOLEAN, () => value === "true")
        .exhaustive();

export const serializeTransformExpr = (expr: TransformExpr): JsonValue =>
  match<TransformExpr, JsonValue>(expr)
    .with({ kind: TransformExprKind.EMPTY }, () => null)
    .with({ kind: TransformExprKind.COLUMN }, ({ name }) => ({ col: name }))
    .with({ kind: TransformExprKind.LITERAL }, serializeTransformLiteral)
    .with({ kind: TransformExprKind.CALL }, (call) => {
      if (call.fn === "") return serializeTransformExpr(call.args[0] ?? TRANSFORM_EMPTY_EXPR);
      const args = trimTrailingEmptyExprs(call.args).map(serializeTransformExpr);
      return { [call.fn]: args.length === 1 ? args[0] : args };
    })
    .exhaustive();

export const serializeTransformStep = (step: TransformStep): JsonValue =>
  match<TransformStep, JsonValue>(step)
    .with({ kind: TransformStepKind.RAW }, ({ json }) => json)
    .with({ kind: TransformStepKind.RENAME }, ({ pairs }) => ({
      rename: Object.fromEntries(pairs.map((pair) => [pair.from, pair.to.trim()])),
    }))
    .with({ kind: TransformStepKind.DROP }, ({ names }) => ({ drop: [...names] }))
    .with({ kind: TransformStepKind.COMPUTE }, ({ outputs, where }): JsonValue => {
      const compute = Object.fromEntries(
        outputs.map((output) => [
          getTransformOutputName(output, where !== null),
          serializeTransformExpr(output.expr),
        ]),
      );
      return where ? { compute, where: serializeTransformExpr(where) } : { compute };
    })
    .exhaustive();

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
