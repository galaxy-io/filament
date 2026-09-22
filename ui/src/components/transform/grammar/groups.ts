import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import { TRANSFORM_CONDITION_JOIN_TO_FUNCTION_MAP } from "@/components/transform/constants";
import { EMPTY_EXPR } from "@/components/transform/grammar/chain";
import {
  TransformConditionJoin,
  type TransformExpr,
  TransformExprKind,
} from "@/components/transform/types";

export enum TransformConditionItemKind {
  CONDITION = "condition",
  GROUP = "group",
}

export interface TransformConditionGroup {
  join: TransformConditionJoin;
  items: TransformConditionItem[];
}

export type TransformConditionItem =
  | { kind: TransformConditionItemKind.CONDITION; expr: TransformExpr; path: string }
  | { kind: TransformConditionItemKind.GROUP; group: TransformConditionGroup; path: string };

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

const itemOf = (
  expr: TransformExpr,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
  path: string,
): TransformConditionItem =>
  joinOf(expr, functionsByName) === undefined
    ? { kind: TransformConditionItemKind.CONDITION, expr, path }
    : {
        kind: TransformConditionItemKind.GROUP,
        group: groupOf(expr, functionsByName, path),
        path,
      };

export const groupOf = (
  where: TransformExpr,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
  path: string,
): TransformConditionGroup => {
  const join = joinOf(where, functionsByName);
  if (join === undefined) {
    return { join: TransformConditionJoin.AND, items: [itemOf(where, functionsByName, path)] };
  }
  const items: TransformConditionItem[] = [];
  const walk = (expr: TransformExpr, exprPath: string) => {
    if (expr.kind === TransformExprKind.CALL && joinOf(expr, functionsByName) === join) {
      walk(expr.args[0], `${exprPath}.${expr.fn}[0]`);
      walk(expr.args[1], `${exprPath}.${expr.fn}[1]`);
      return;
    }
    items.push(itemOf(expr, functionsByName, exprPath));
  };
  walk(where, path);
  return { join, items };
};

export const groupToExpr = (group: TransformConditionGroup): TransformExpr => {
  const fn = TRANSFORM_CONDITION_JOIN_TO_FUNCTION_MAP[group.join];
  const parts = group.items.map((item) =>
    item.kind === TransformConditionItemKind.GROUP ? groupToExpr(item.group) : item.expr,
  );
  return (
    parts.reduce<TransformExpr | null>(
      (joined, part) =>
        joined === null ? part : { kind: TransformExprKind.CALL, fn, args: [joined, part] },
      null,
    ) ?? EMPTY_EXPR
  );
};

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
    item.kind === TransformConditionItemKind.GROUP
      ? isTransformGroupCompact(item.group, functionsByName)
      : isCompactCondition(item.expr, functionsByName),
  );

export const createTransformConditionGroup = (
  parentJoin: TransformConditionJoin,
): TransformConditionGroup => ({
  join:
    parentJoin === TransformConditionJoin.AND
      ? TransformConditionJoin.OR
      : TransformConditionJoin.AND,
  items: [
    { kind: TransformConditionItemKind.CONDITION, expr: EMPTY_EXPR, path: "" },
    { kind: TransformConditionItemKind.CONDITION, expr: EMPTY_EXPR, path: "" },
  ],
});
