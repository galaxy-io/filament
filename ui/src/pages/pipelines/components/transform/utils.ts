import { DataType, DateUnit, Precision, type Schema, TimeUnit, tableFromIPC } from "apache-arrow";
import { match } from "ts-pattern";

import { type AlchemyFlowContract, createAlchemyFunctionHref } from "@galaxy-io/alchemy-ui/links";
import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";

import { AlchemyContractKind, type AlchemyFunction } from "@/gen/ingestion/v1/alchemy_pb";
import type { ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import {
  TRANSFORM_ALCHEMY_FUNCTIONS_PATH,
  TRANSFORM_ALCHEMY_REQUEST_PARAM,
  TRANSFORM_ALCHEMY_RETURN_LABEL,
  TRANSFORM_ALCHEMY_WORKBENCH_FILE_PARAM,
  TRANSFORM_ALCHEMY_WORKBENCH_PATH,
  TRANSFORM_ALCHEMY_WORKBENCH_SOURCE_PREFIX,
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
import {
  type PipelineTransformFieldsDraft,
  type PipelineTransformFieldsEditor,
  type TransformAction,
  TransformActionKind,
  type TransformAlchemyReturnMessage,
  type TransformChain,
  type TransformChainCall,
  type TransformComputeStep,
  type TransformEditableStep,
  type TransformExpr,
  TransformExprKind,
  type TransformLeafExpr,
  type TransformLiteralExpr,
  TransformLiteralKind,
  type TransformSelectEntry,
  type TransformSelectModel,
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
  id: string = crypto.randomUUID(),
): TransformEditableStep =>
  match<TransformEditableStep["kind"], TransformEditableStep>(kind)
    .with(TransformStepKind.RENAME, (kind) => ({ id, kind, pairs: [{ from: column, to: "" }] }))
    .with(TransformStepKind.DROP, (kind) => ({ id, kind, names: [column] }))
    .with(TransformStepKind.COMPUTE, () => createTransformComputeStep(column, id))
    .with(TransformStepKind.ALCHEMY, (kind) => ({ id, kind, function: "", version: 0 }))
    .exhaustive();

export const getTransformStepColumn = (step: TransformEditableStep): string =>
  match(step)
    .with({ kind: TransformStepKind.RENAME }, (rename) => rename.pairs[0]?.from ?? "")
    .with({ kind: TransformStepKind.DROP }, (drop) => drop.names[0] ?? "")
    .with(
      { kind: TransformStepKind.COMPUTE },
      (compute) => getTransformRootColumn(compute.outputs[0]?.expr ?? TRANSFORM_EMPTY_EXPR) ?? "",
    )
    .with({ kind: TransformStepKind.ALCHEMY }, () => "")
    .exhaustive();

const convertTransformStep = (
  step: TransformEditableStep,
  kind: TransformEditableStep["kind"],
): TransformEditableStep =>
  step.kind === kind ? step : createTransformStep(kind, getTransformStepColumn(step), step.id);

const toTransformComputeStep = (step: TransformEditableStep): TransformComputeStep =>
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

const createTransformCallArgs = (
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
    const literalCount =
      literalKind === undefined ? 0 : literalKind === TransformLiteralKind.BOOLEAN ? 2 : 1;
    const accepted = getTransformAcceptedColumns(columns, types);
    if (literalCount + accepted.length !== 1) return TRANSFORM_EMPTY_EXPR;
    if (literalKind !== undefined) return createTransformLiteral(literalKind);
    return accepted[0].name === rootColumn
      ? TRANSFORM_EMPTY_EXPR
      : createTransformColumnExpr(accepted[0].name);
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
  if (action.kind === TransformActionKind.ALCHEMY) {
    return convertTransformStep(step, TransformStepKind.ALCHEMY);
  }
  const compute = toTransformComputeStep(step);
  const output = compute.outputs[outputIndex];
  if (!output) return compute;
  const chain = getTransformChain(output.expr);
  const acceptedTypes =
    action.kind === TransformActionKind.FUNCTION
      ? (functionsByName.get(action.fn)?.args[0]?.logicalTypes ?? [])
      : [];
  const accepted = getTransformAcceptedColumns(columns, acceptedTypes);
  const filled =
    chain.root.kind === TransformExprKind.EMPTY && accepted.length === 1
      ? accepted[0].name
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
  steps: TransformStep[],
  draft: PipelineTransformFieldsDraft,
): { steps: TransformStep[]; index: number } => {
  const at = draft.id === null ? -1 : steps.findIndex((step) => step.id === draft.id);
  return at === -1
    ? { steps: [...steps, draft.step], index: steps.length }
    : { steps: steps.map((step, slot) => (slot === at ? draft.step : step)), index: at };
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
    output && outputType === undefined
      ? ""
      : outputType === undefined
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

export const createTransformFunctionOption = (fn: TransformFunction): SelectOption => ({
  id: fn.name,
  label: fn.displayName || fn.name,
  icon: TRANSFORM_FUNCTION_TO_ICON_MAP.get(fn.name),
});

export const createTransformBooleanOptions = (): SelectOption[] =>
  ["true", "false"].map((value) => ({
    id: value,
    label: value,
    icon: TRANSFORM_LITERAL_KIND_TO_ICON_MAP[TransformLiteralKind.BOOLEAN],
  }));

export const getAlchemyFunctionDisabledReason = (fn: AlchemyFunction): string | undefined => {
  if (fn.contractKind !== AlchemyContractKind.ARROW) return "Needs an Arrow contract";
  if (fn.currentVersion === 0n) return "No version is ready yet";
  return undefined;
};

export const hasAlchemyRuntimeIcon = (fn: {
  runtimeDarkIconUrl: string;
  runtimeLightIconUrl: string;
}): boolean => fn.runtimeDarkIconUrl !== "" && fn.runtimeLightIconUrl !== "";

const createAlchemyUrl = (alchemyUrl: string, path: string): URL =>
  new URL(path, alchemyUrl.endsWith("/") ? alchemyUrl : `${alchemyUrl}/`);

export const getAlchemyWorkbenchUrl = (
  alchemyUrl: string,
  functionId: string,
  handlerPath: string | undefined,
): string => {
  const url = createAlchemyUrl(
    alchemyUrl,
    `${TRANSFORM_ALCHEMY_FUNCTIONS_PATH}/${encodeURIComponent(functionId)}/${TRANSFORM_ALCHEMY_WORKBENCH_PATH}`,
  );
  if (handlerPath) {
    url.searchParams.set(
      TRANSFORM_ALCHEMY_WORKBENCH_FILE_PARAM,
      `${TRANSFORM_ALCHEMY_WORKBENCH_SOURCE_PREFIX}${handlerPath}`,
    );
  }
  return url.toString();
};

const ALCHEMY_TIME_UNIT_TO_TOKEN_MAP: Record<TimeUnit, string> = {
  [TimeUnit.SECOND]: "s",
  [TimeUnit.MILLISECOND]: "ms",
  [TimeUnit.MICROSECOND]: "us",
  [TimeUnit.NANOSECOND]: "ns",
};

const ALCHEMY_PRECISION_TO_BIT_WIDTH_MAP: Record<Precision, number> = {
  [Precision.HALF]: 16,
  [Precision.SINGLE]: 32,
  [Precision.DOUBLE]: 64,
};

const formatAlchemyArrowType = (type: DataType): string => {
  if (DataType.isUtf8(type)) return "utf8";
  if (DataType.isLargeUtf8(type)) return "large_utf8";
  if (DataType.isBinary(type)) return "binary";
  if (DataType.isLargeBinary(type)) return "large_binary";
  if (DataType.isBool(type)) return "bool";
  if (DataType.isInt(type)) return `${type.isSigned ? "int" : "uint"}${type.bitWidth}`;
  if (DataType.isFloat(type)) return `float${ALCHEMY_PRECISION_TO_BIT_WIDTH_MAP[type.precision]}`;
  if (DataType.isDate(type)) return type.unit === DateUnit.DAY ? "date32" : "date64";
  if (DataType.isTimestamp(type)) {
    const unit = ALCHEMY_TIME_UNIT_TO_TOKEN_MAP[type.unit];
    return type.timezone ? `timestamp[${unit}, tz=${type.timezone}]` : `timestamp[${unit}]`;
  }
  if (DataType.isTime(type)) {
    return `time${type.bitWidth}[${ALCHEMY_TIME_UNIT_TO_TOKEN_MAP[type.unit]}]`;
  }
  if (DataType.isDecimal(type)) return `decimal${type.bitWidth}(${type.precision}, ${type.scale})`;
  if (DataType.isList(type)) return `list<${formatAlchemyArrowType(type.valueType)}>`;
  if (DataType.isFixedSizeList(type)) {
    return `fixed_size_list<${formatAlchemyArrowType(type.valueType)}>[${type.listSize}]`;
  }
  return String(type).toLowerCase();
};

const getAlchemyArrowFields = (schema: Schema) =>
  schema.fields.map((field) => ({
    name: field.name,
    type: formatAlchemyArrowType(field.type),
    nullable: field.nullable,
  }));

export const getAlchemyArrowContract = (
  arrowSchema: Uint8Array | undefined,
): AlchemyFlowContract | undefined => {
  if (!arrowSchema?.length) return undefined;
  try {
    const fields = getAlchemyArrowFields(tableFromIPC(arrowSchema).schema);
    return { kind: "arrow", input: fields, output: fields };
  } catch {
    return undefined;
  }
};

export const getAlchemyCreateFunctionHref = (
  alchemyUrl: string,
  returnTo: string,
  request: string,
  contract: AlchemyFlowContract | undefined,
): string => {
  const url = new URL(returnTo);
  url.searchParams.set(TRANSFORM_ALCHEMY_REQUEST_PARAM, request);
  return createAlchemyFunctionHref(alchemyUrl, url.toString(), {
    contract,
    returnLabel: TRANSFORM_ALCHEMY_RETURN_LABEL,
  });
};

export const parseAlchemyReturnMessage = (
  data: unknown,
): TransformAlchemyReturnMessage | undefined => {
  if (typeof data !== "object" || data === null) return undefined;
  const { request, functionId, version } = data as Record<string, unknown>;
  return typeof request === "string" &&
    typeof functionId === "string" &&
    typeof version === "number"
    ? { request, functionId, version }
    : undefined;
};

export const createTransformSelectModel = <T>(
  entries: TransformSelectEntry<T>[],
): TransformSelectModel<T> => ({
  options: entries.map((entry) => entry.option),
  payloadById: new Map(entries.map((entry) => [entry.option.id, entry.payload])),
});
