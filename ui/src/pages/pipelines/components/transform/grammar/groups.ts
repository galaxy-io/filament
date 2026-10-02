import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import { TRANSFORM_CONDITION_JOIN_TO_OPPOSITE_MAP } from "@/pages/pipelines/components/transform/constants";
import { TRANSFORM_EMPTY_EXPR } from "@/pages/pipelines/components/transform/grammar/chain";
import {
  TransformConditionJoin,
  type TransformExpr,
  TransformExprKind,
} from "@/pages/pipelines/components/transform/types";

export interface TransformConditionItem {
  expr: TransformExpr;
  path: string;
}

export interface TransformConditionGroup {
  join: TransformConditionJoin;
  items: TransformConditionItem[];
}

export const isTransformConditionOperator = (fn: TransformFunction | undefined): boolean =>
  fn?.conditionOperator === true;

export const getTransformConditionJoin = (
  fn: TransformFunction | undefined,
): TransformConditionJoin | undefined =>
  Object.values(TransformConditionJoin).find((join) => join === fn?.conditionJoin);

const joinOf = (
  expr: TransformExpr,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): TransformConditionJoin | undefined =>
  expr.kind === TransformExprKind.CALL && expr.args.length === 2
    ? getTransformConditionJoin(functionsByName.get(expr.fn))
    : undefined;

export const isTransformConditionGroupExpr = (
  expr: TransformExpr,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): boolean => joinOf(expr, functionsByName) !== undefined;

export const getTransformConditionGroup = (
  where: TransformExpr,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
  path: string,
): TransformConditionGroup => {
  const join = joinOf(where, functionsByName) ?? TransformConditionJoin.AND;
  const items: TransformConditionItem[] = [];
  const walk = (expr: TransformExpr, exprPath: string) => {
    if (expr.kind === TransformExprKind.CALL && joinOf(expr, functionsByName) === join) {
      walk(expr.args[0], `${exprPath}.${expr.fn}[0]`);
      walk(expr.args[1], `${exprPath}.${expr.fn}[1]`);
      return;
    }
    items.push({ expr, path: exprPath });
  };
  walk(where, path);
  return { join, items };
};

export const createTransformConditionExpr = (group: TransformConditionGroup): TransformExpr =>
  group.items.reduce<TransformExpr | null>(
    (joined, item) =>
      joined === null
        ? item.expr
        : { kind: TransformExprKind.CALL, fn: group.join, args: [joined, item.expr] },
    null,
  ) ?? TRANSFORM_EMPTY_EXPR;

const isCompactCondition = (
  expr: TransformExpr,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): boolean => {
  if (expr.kind === TransformExprKind.EMPTY || expr.kind === TransformExprKind.COLUMN) return true;
  if (expr.kind !== TransformExprKind.CALL) return false;
  if (!isTransformConditionOperator(functionsByName.get(expr.fn))) return false;
  const [input, ...rest] = expr.args;
  return (
    input?.kind === TransformExprKind.COLUMN &&
    rest.every((arg) => arg.kind !== TransformExprKind.CALL)
  );
};

export const isTransformGroupCompact = (
  group: TransformConditionGroup,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): boolean =>
  group.items.every((item) =>
    isTransformConditionGroupExpr(item.expr, functionsByName)
      ? isTransformGroupCompact(
          getTransformConditionGroup(item.expr, functionsByName, item.path),
          functionsByName,
        )
      : isCompactCondition(item.expr, functionsByName),
  );

export const createTransformConditionGroupExpr = (
  parentJoin: TransformConditionJoin,
): TransformExpr => ({
  kind: TransformExprKind.CALL,
  fn: TRANSFORM_CONDITION_JOIN_TO_OPPOSITE_MAP[parentJoin],
  args: [TRANSFORM_EMPTY_EXPR, TRANSFORM_EMPTY_EXPR],
});
