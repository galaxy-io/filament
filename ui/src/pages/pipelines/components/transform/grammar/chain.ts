import {
  type TransformChain,
  type TransformChainCall,
  type TransformExpr,
  TransformExprKind,
  type TransformLeafExpr,
} from "@/pages/pipelines/components/transform/types";

export const TRANSFORM_EMPTY_EXPR: TransformLeafExpr = { kind: TransformExprKind.EMPTY };

export const createTransformColumnExpr = (name: string): TransformLeafExpr =>
  name === "" ? TRANSFORM_EMPTY_EXPR : { kind: TransformExprKind.COLUMN, name };

export const trimTrailingEmptyExprs = (args: TransformExpr[]): TransformExpr[] => {
  let last = args.length - 1;
  while (last >= 0 && args[last].kind === TransformExprKind.EMPTY) last--;
  return args.slice(0, last + 1);
};

export const getTransformChain = (expr: TransformExpr): TransformChain => {
  if (expr.kind !== TransformExprKind.CALL) return { root: expr, calls: [] };
  const [input = TRANSFORM_EMPTY_EXPR, ...rest] = expr.args;
  const inner = getTransformChain(input);
  return { root: inner.root, calls: [...inner.calls, { fn: expr.fn, args: rest }] };
};

export const createTransformChainExpr = (
  root: TransformLeafExpr,
  calls: TransformChainCall[],
): TransformExpr =>
  calls.reduce<TransformExpr>(
    (inner, call) => ({ kind: TransformExprKind.CALL, fn: call.fn, args: [inner, ...call.args] }),
    root,
  );

export const getTransformChainInput = (chain: TransformChain, index: number): TransformExpr =>
  createTransformChainExpr(chain.root, chain.calls.slice(0, index));

export const getTransformRootColumn = (expr: TransformExpr): string | undefined => {
  const root = getTransformChain(expr).root;
  return root.kind === TransformExprKind.COLUMN ? root.name : undefined;
};
