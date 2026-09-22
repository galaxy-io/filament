import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import { TRANSFORM_RAW_STEP_LABEL } from "@/pages/pipelines/components/transform/constants";
import { trimTrailingEmptyExprs } from "@/pages/pipelines/components/transform/grammar/chain";
import {
  getTransformOutputName,
  isTransformOutputInPlace,
} from "@/pages/pipelines/components/transform/grammar/serialize";
import {
  type TransformExpr,
  TransformExprKind,
  TransformLiteralKind,
  type TransformStep,
  TransformStepKind,
} from "@/pages/pipelines/components/transform/types";

export enum TransformSummaryPartKind {
  TEXT = "text",
  VERB = "verb",
  COLUMN = "column",
  OUTPUT = "output",
}

export interface TransformSummaryPart {
  kind: TransformSummaryPartKind;
  text: string;
}

const TRANSFORM_EXPR_KIND_TO_VERB_MAP: Record<TransformExprKind, string | undefined> = {
  [TransformExprKind.EMPTY]: "Duplicate",
  [TransformExprKind.COLUMN]: "Duplicate",
  [TransformExprKind.LITERAL]: "Set",
  [TransformExprKind.CALL]: undefined,
};

const text = (value: string): TransformSummaryPart => ({
  kind: TransformSummaryPartKind.TEXT,
  text: value,
});
const verb = (value: string): TransformSummaryPart => ({
  kind: TransformSummaryPartKind.VERB,
  text: value,
});
const column = (name: string): TransformSummaryPart => ({
  kind: TransformSummaryPartKind.COLUMN,
  text: name,
});
const output = (name: string): TransformSummaryPart => ({
  kind: TransformSummaryPartKind.OUTPUT,
  text: name,
});
const ELLIPSIS = text("…");

const joinParts = (parts: TransformSummaryPart[]): TransformSummaryPart[] =>
  parts.reduce<TransformSummaryPart[]>((merged, part) => {
    const last = merged[merged.length - 1];
    if (
      part.kind === TransformSummaryPartKind.TEXT &&
      last?.kind === TransformSummaryPartKind.TEXT
    ) {
      merged[merged.length - 1] = text(`${last.text}${part.text}`);
    } else {
      merged.push(part);
    }
    return merged;
  }, []);

const withSeparator = (
  items: TransformSummaryPart[][],
  separator: string,
): TransformSummaryPart[] =>
  items.flatMap((item, index) => (index === 0 ? item : [text(separator), ...item]));

const formatLiteral = (literalKind: TransformLiteralKind, value: string): string =>
  literalKind === TransformLiteralKind.STRING ? JSON.stringify(value) : value;

const isInfix = (fn: TransformFunction | undefined): boolean =>
  Boolean(fn?.operatorSymbol || fn?.conditionJoin);

const needsParentheses = (
  expr: TransformExpr,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): boolean =>
  expr.kind === TransformExprKind.CALL &&
  (isInfix(functionsByName.get(expr.fn)) || expr.args[0]?.kind === TransformExprKind.CALL);

export const formatTransformExpr = (
  expr: TransformExpr,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): TransformSummaryPart[] => {
  switch (expr.kind) {
    case TransformExprKind.EMPTY:
      return [ELLIPSIS];
    case TransformExprKind.COLUMN:
      return [column(expr.name)];
    case TransformExprKind.LITERAL:
      return [expr.value === null ? ELLIPSIS : text(formatLiteral(expr.literalKind, expr.value))];
    case TransformExprKind.CALL: {
      const fn = functionsByName.get(expr.fn);
      const operand = (arg: TransformExpr): TransformSummaryPart[] =>
        needsParentheses(arg, functionsByName)
          ? [text("("), ...formatTransformExpr(arg, functionsByName), text(")")]
          : formatTransformExpr(arg, functionsByName);
      if (fn?.conditionJoin) {
        const joined = expr.args.map((arg) =>
          arg.kind === TransformExprKind.CALL && functionsByName.get(arg.fn)?.conditionJoin
            ? [text("("), ...formatTransformExpr(arg, functionsByName), text(")")]
            : formatTransformExpr(arg, functionsByName),
        );
        return withSeparator(joined, ` ${fn.conditionJoin} `);
      }
      const [input, ...rest] = expr.args;
      const args = trimTrailingEmptyExprs(rest).map(operand);
      const head = input ? operand(input) : [ELLIPSIS];
      if (fn?.operatorSymbol) {
        return [...head, text(` ${fn.operatorSymbol} `), ...(args[0] ?? [ELLIPSIS])];
      }
      const name = verb(fn ? fn.displayName || fn.name : expr.fn || "…");
      const callArgs = args.length > 0 ? [text(" ("), ...withSeparator(args, ", "), text(")")] : [];
      if (input?.kind === TransformExprKind.CALL) {
        return [...formatTransformExpr(input, functionsByName), text(", then "), name, ...callArgs];
      }
      return [name, text(" "), ...head, ...callArgs];
    }
  }
};

export const formatTransformStep = (
  step: TransformStep,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): TransformSummaryPart[] => {
  switch (step.kind) {
    case TransformStepKind.RAW:
      return [text(TRANSFORM_RAW_STEP_LABEL)];
    case TransformStepKind.RENAME:
      return joinParts([
        verb("Rename"),
        text(" "),
        ...withSeparator(
          step.pairs.map((pair) => [
            column(pair.from || "…"),
            text(" to "),
            pair.to ? output(pair.to) : ELLIPSIS,
          ]),
          " and ",
        ),
      ]);
    case TransformStepKind.DROP:
      return joinParts([
        verb("Drop"),
        text(" "),
        ...withSeparator(
          step.names.map((name) => [column(name || "…")]),
          " and ",
        ),
      ]);
    case TransformStepKind.COMPUTE: {
      const isMatchingRows = step.where !== null;
      const outputs = step.outputs.map((entry) => {
        const rootVerb = TRANSFORM_EXPR_KIND_TO_VERB_MAP[entry.expr.kind];
        const head = rootVerb === undefined ? [] : [verb(rootVerb), text(" ")];
        const named = isTransformOutputInPlace(entry, isMatchingRows)
          ? []
          : [text(" as "), output(getTransformOutputName(entry, isMatchingRows))];
        return [...head, ...formatTransformExpr(entry.expr, functionsByName), ...named];
      });
      const where = step.where
        ? [text(" on rows where "), ...formatTransformExpr(step.where, functionsByName)]
        : [];
      return joinParts([...withSeparator(outputs, ", and "), ...where]);
    }
  }
};
