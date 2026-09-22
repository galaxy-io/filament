import { createElement } from "react";

import { match } from "ts-pattern";

import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import {
  TRANSFORM_FUNCTION_TO_ICON_MAP,
  TRANSFORM_LITERAL_KIND_TO_ICON_MAP,
} from "@/pages/pipelines/components/transform/constants";
import {
  getTransformAcceptedColumns,
  getTransformArgumentSpec,
  getTransformArgumentTypes,
  getTransformCallSlotCount,
  getTransformLiteralKind,
  isTransformExprComplete,
} from "@/pages/pipelines/components/transform/grammar/catalog";
import {
  createTransformChainExpr,
  createTransformColumnExpr,
  getTransformChain,
  getTransformRootColumn,
  TRANSFORM_EMPTY_EXPR,
} from "@/pages/pipelines/components/transform/grammar/chain";
import { getTransformOutputPath } from "@/pages/pipelines/components/transform/grammar/paths";
import {
  getTransformOutputName,
  isTransformOutputInPlace,
} from "@/pages/pipelines/components/transform/grammar/serialize";
import PipelineTransformFieldsOptionIcon from "@/pages/pipelines/components/transform/PipelineTransformFieldsOptionIcon";
import {
  type PipelineTransformFieldsDraft,
  type PipelineTransformFieldsEditor,
  type TransformAction,
  TransformActionKind,
  type TransformChain,
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
} from "@/pages/pipelines/components/transform/types";

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
  return match<TransformEditableStep["kind"], TransformEditableStep>(kind)
    .with(TransformStepKind.RENAME, (kind) => ({
      id: crypto.randomUUID(),
      kind,
      pairs: [{ from: column, to: "" }],
    }))
    .with(TransformStepKind.DROP, (kind) => ({ id: crypto.randomUUID(), kind, names: [column] }))
    .with(TransformStepKind.COMPUTE, () => createTransformComputeStep(column))
    .exhaustive();
};

export const getTransformStepColumn = (step: TransformEditableStep): string =>
  match(step)
    .with({ kind: TransformStepKind.RENAME }, (rename) => rename.pairs[0]?.from ?? "")
    .with({ kind: TransformStepKind.DROP }, (drop) => drop.names[0] ?? "")
    .with(
      { kind: TransformStepKind.COMPUTE },
      (compute) => getTransformRootColumn(compute.outputs[0]?.expr ?? TRANSFORM_EMPTY_EXPR) ?? "",
    )
    .exhaustive();

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

export const getTransformOnlyColumn = (
  columns: ResourceColumn[],
  logicalTypes: string[],
  exclude: string | undefined,
): ResourceColumn["name"] | undefined => {
  const accepted = getTransformAcceptedColumns(columns, logicalTypes);
  return accepted.length === 1 && accepted[0].name !== exclude ? accepted[0].name : undefined;
};

export const createTransformCallArgs = (
  fn: TransformFunction,
  inputType: string | undefined,
  columns: ResourceColumn[],
  rootColumn: string | undefined,
): TransformExpr[] =>
  Array.from({ length: getTransformCallSlotCount(fn) }, (_, position) => {
    const spec = getTransformArgumentSpec(fn, position + 1);
    if (!spec || spec.isLiteral || spec.isOptional) return TRANSFORM_EMPTY_EXPR;
    const types = getTransformArgumentTypes(fn, position + 1, inputType);
    const literalKind = spec.isColumn ? undefined : getTransformLiteralKind(types);
    if (literalKind !== undefined && literalKind !== TransformLiteralKind.BOOLEAN) {
      return getTransformAcceptedColumns(columns, types).length === 0
        ? createTransformLiteral(literalKind)
        : TRANSFORM_EMPTY_EXPR;
    }
    const only =
      literalKind === undefined ? getTransformOnlyColumn(columns, types, rootColumn) : undefined;
    return only === undefined ? TRANSFORM_EMPTY_EXPR : createTransformColumnExpr(only);
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

export const getTransformChainAction = (chain: TransformChain): TransformAction => {
  const first = chain.calls[0];
  if (first) return { kind: TransformActionKind.FUNCTION, fn: first.fn };
  if (chain.root.kind === TransformExprKind.LITERAL) {
    return { kind: TransformActionKind.LITERAL, literalKind: chain.root.literalKind };
  }
  return { kind: TransformActionKind.COPY };
};

export const getTransformActionId = (action: TransformAction): string =>
  match(action)
    .with(
      { kind: TransformActionKind.LITERAL },
      ({ kind, literalKind }) => `${kind}:${literalKind}`,
    )
    .with({ kind: TransformActionKind.FUNCTION }, ({ kind, fn }) => `${kind}:${fn}`)
    .otherwise(({ kind }) => kind);

interface TransformActionContext {
  functionsByName: Map<TransformFunction["name"], TransformFunction>;
  columns: ResourceColumn[];
  rootType: string | undefined;
}

export const applyTransformAction = (
  step: TransformEditableStep,
  outputIndex: number,
  action: TransformAction,
  { functionsByName, columns, rootType }: TransformActionContext,
): TransformEditableStep => {
  if (action.kind === TransformActionKind.RENAME) {
    return convertTransformStep(step, TransformStepKind.RENAME);
  }
  if (action.kind === TransformActionKind.DROP) {
    return convertTransformStep(step, TransformStepKind.DROP);
  }
  const compute = toTransformComputeStep(step);
  const output = compute.outputs[outputIndex];
  if (!output) return compute;
  const chain = getTransformChain(output.expr);
  const acceptedTypes =
    action.kind === TransformActionKind.FUNCTION
      ? (functionsByName.get(action.fn)?.args[0]?.logicalTypes ?? [])
      : [];
  const filled =
    chain.root.kind === TransformExprKind.EMPTY && action.kind !== TransformActionKind.LITERAL
      ? getTransformOnlyColumn(columns, acceptedTypes, undefined)
      : undefined;
  const root: TransformLeafExpr =
    filled === undefined ? chain.root : createTransformColumnExpr(filled);
  const rootColumn = root.kind === TransformExprKind.COLUMN ? root.name : undefined;
  const expr = match(action)
    .with({ kind: TransformActionKind.COPY }, () =>
      createTransformChainExpr(
        root.kind === TransformExprKind.COLUMN ? root : TRANSFORM_EMPTY_EXPR,
        [],
      ),
    )
    .with({ kind: TransformActionKind.LITERAL }, ({ literalKind }) =>
      createTransformLiteral(literalKind),
    )
    .with({ kind: TransformActionKind.FUNCTION }, ({ fn }) =>
      createTransformChainExpr(
        root,
        replaceTransformChainCall(
          chain.calls,
          0,
          fn,
          filled === undefined ? rootType : getTransformColumnType(columns, filled),
          functionsByName,
          columns,
          rootColumn,
        ),
      ),
    )
    .exhaustive();
  return {
    ...compute,
    outputs: compute.outputs.map((candidate, index) =>
      index === outputIndex ? { ...candidate, expr } : candidate,
    ),
  };
};

export const placeTransformDraft = (
  stepsByResource: Map<Resource["name"], TransformStep[]>,
  draft: PipelineTransformFieldsDraft,
): { stepsByResource: Map<Resource["name"], TransformStep[]>; index: number } => {
  const steps = stepsByResource.get(draft.resource) ?? [];
  const at = draft.id === null ? -1 : steps.findIndex((step) => step.id === draft.id);
  const index = at === -1 ? steps.length : at;
  const next = new Map(stepsByResource);
  next.set(
    draft.resource,
    at === -1
      ? [...steps, draft.step]
      : steps.map((step, slot) => (slot === at ? draft.step : step)),
  );
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

export const getTransformNumberError = (value: string, isIntegerOnly: boolean): string | null => {
  const parsed = value.trim() === "" ? Number.NaN : Number(value);
  if (!Number.isFinite(parsed)) return "Enter a valid number.";
  if (isIntegerOnly && !Number.isSafeInteger(parsed)) return "Enter a whole number.";
  return null;
};

export const getTransformTypeSummary = (
  draft: PipelineTransformFieldsDraft,
  editor: PipelineTransformFieldsEditor,
  functionsByName: Map<TransformFunction["name"], TransformFunction>,
): { typeSummary: string; warning: string | null } => {
  const columnType =
    getTransformColumnType(editor.columns, getTransformStepColumn(draft.step)) ?? "";
  const output = draft.step.kind === TransformStepKind.COMPUTE ? draft.step.outputs[0] : undefined;
  const isMatchingRows = draft.step.kind === TransformStepKind.COMPUTE && draft.step.where !== null;
  const outputType =
    output && isTransformExprComplete(output.expr, functionsByName)
      ? getTransformExprType(output.expr, getTransformOutputPath(output, isMatchingRows), editor)
      : undefined;
  const isChanged = outputType !== undefined && columnType !== "" && columnType !== outputType;
  const typeSummary =
    outputType === undefined
      ? columnType
      : isChanged
        ? `${columnType} → ${outputType}`
        : outputType;
  const warning =
    output && isChanged && !isMatchingRows && isTransformOutputInPlace(output, false)
      ? `${getTransformOutputName(output, false)} is ${columnType}; this expression returns ${outputType}.`
      : null;
  return { typeSummary, warning };
};

export const filterTransformOptions = (
  term: string,
  options: SelectInputOption[],
): SelectInputOption[] =>
  options.filter((option) => option.label.toLowerCase().includes(term.toLowerCase()));

export const createTransformFunctionOption = (fn: TransformFunction): SelectInputOption => {
  const icon = TRANSFORM_FUNCTION_TO_ICON_MAP.get(fn.name);
  return {
    id: fn.name,
    label: fn.displayName || fn.name,
    value: fn.name,
    icon: icon ? createElement(PipelineTransformFieldsOptionIcon, { icon }) : undefined,
  };
};

export const createTransformBooleanOptions = (): SelectInputOption[] =>
  ["true", "false"].map((value) => ({
    id: value,
    label: value,
    value: { kind: TransformExprKind.LITERAL, literalKind: TransformLiteralKind.BOOLEAN, value },
    icon: createElement(PipelineTransformFieldsOptionIcon, {
      icon: TRANSFORM_LITERAL_KIND_TO_ICON_MAP[TransformLiteralKind.BOOLEAN],
    }),
  }));
