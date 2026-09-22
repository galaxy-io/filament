import type { ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import { chainOf, EMPTY_EXPR } from "@/components/transform/grammar/chain";
import { getTransformOutputName } from "@/components/transform/grammar/serialize";
import {
  type PipelineTransformFieldsDraft,
  type TransformExpr,
  TransformExprKind,
  TransformLiteralKind,
  TransformStepKind,
} from "@/components/transform/types";
import {
  getTransformArgumentSpec,
  getTransformColumnType,
  getTransformNumberError,
  isTransformExprComplete,
  isTransformIntegerOnly,
} from "@/components/transform/utils";

export const isTransformDraftComplete = (
  draft: PipelineTransformFieldsDraft,
  columns: ResourceColumn[],
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): boolean => {
  if (draft.resource === "") return false;
  const step = draft.step;
  switch (step.kind) {
    case TransformStepKind.RENAME:
      return (
        step.pairs.length > 0 &&
        step.pairs.every((pair) => pair.from !== "" && pair.to.trim() !== "")
      );
    case TransformStepKind.DROP:
      return step.names.length > 0 && step.names.every((name) => name !== "");
    case TransformStepKind.COMPUTE: {
      if (step.outputs.length === 0) return false;
      if (!step.outputs.every((output) => isTransformExprComplete(output.expr, functionsByName))) {
        return false;
      }
      if (step.where === null) return true;
      if (!isTransformExprComplete(step.where, functionsByName)) return false;
      const { root, calls } = chainOf(step.where);
      return (
        calls.length > 0 ||
        root.kind !== TransformExprKind.COLUMN ||
        getTransformColumnType(columns, root.name) === "bool"
      );
    }
  }
};

const getTransformExprError = (
  expr: TransformExpr,
  integerOnly: boolean,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): string | null => {
  if (expr.kind === TransformExprKind.LITERAL) {
    return expr.literalKind === TransformLiteralKind.NUMBER && expr.value !== null
      ? getTransformNumberError(expr.value, integerOnly)
      : null;
  }
  if (expr.kind !== TransformExprKind.CALL) return null;
  const fn = functionsByName.get(expr.fn);
  for (const [position, arg] of expr.args.entries()) {
    const spec = fn ? getTransformArgumentSpec(fn, position) : undefined;
    const error = getTransformExprError(
      arg,
      isTransformIntegerOnly(spec?.logicalTypes ?? []),
      functionsByName,
    );
    if (error !== null) return error;
  }
  return null;
};

const findDuplicate = (names: string[]): string | undefined =>
  names.find((name, index) => names.indexOf(name) !== index);

export const getTransformDraftError = (
  draft: PipelineTransformFieldsDraft,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): string | null => {
  const step = draft.step;
  switch (step.kind) {
    case TransformStepKind.RENAME: {
      const same = step.pairs.find((pair) => pair.from !== "" && pair.to.trim() === pair.from);
      if (same) return "The new column name must be different.";
      const source = findDuplicate(step.pairs.map((pair) => pair.from).filter(Boolean));
      if (source !== undefined) return `Column ${source} is renamed more than once.`;
      const target = findDuplicate(step.pairs.map((pair) => pair.to.trim()).filter(Boolean));
      return target === undefined ? null : `Column ${target} is used more than once.`;
    }
    case TransformStepKind.DROP: {
      const name = findDuplicate(step.names.filter(Boolean));
      return name === undefined ? null : `Column ${name} is selected more than once.`;
    }
    case TransformStepKind.COMPUTE: {
      const isMatchingRows = step.where !== null;
      const names = step.outputs.map((output) => getTransformOutputName(output, isMatchingRows));
      const name = findDuplicate(names.filter(Boolean));
      if (name !== undefined) return `Output column ${name} is used more than once.`;
      const identity = step.outputs.find(
        (output, index) =>
          output.expr.kind === TransformExprKind.COLUMN && names[index] === output.expr.name,
      );
      if (identity) return "Choose a function or save the result to another column.";
      for (const expr of [...step.outputs.map((output) => output.expr), step.where ?? EMPTY_EXPR]) {
        const error = getTransformExprError(expr, false, functionsByName);
        if (error !== null) return error;
      }
      return null;
    }
  }
};
