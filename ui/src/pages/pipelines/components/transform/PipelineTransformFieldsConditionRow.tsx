import type { ReactNode } from "react";

import { styled } from "@linaria/react";

import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import {
  TRANSFORM_CONDITION_STACK_WIDTH,
  TRANSFORM_GAP,
  TRANSFORM_SELECT_ERROR_MARK,
} from "@/pages/pipelines/components/transform/constants";
import {
  getCompatibleTransformFunctions,
  getTransformArgumentSpec,
  getTransformArgumentTypes,
} from "@/pages/pipelines/components/transform/grammar/catalog";
import {
  createTransformChainExpr,
  getTransformChain,
  TRANSFORM_EMPTY_EXPR,
} from "@/pages/pipelines/components/transform/grammar/chain";
import { isTransformConditionOperator } from "@/pages/pipelines/components/transform/grammar/groups";
import {
  getTransformArgumentPath,
  getTransformInputPath,
} from "@/pages/pipelines/components/transform/grammar/paths";
import PipelineTransformFieldsLeaf from "@/pages/pipelines/components/transform/PipelineTransformFieldsLeaf";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsRow";
import {
  type TransformExpr,
  TransformExprKind,
  type TransformLeafExpr,
} from "@/pages/pipelines/components/transform/types";
import {
  createTransformChainCall,
  createTransformFunctionOption,
  getTransformColumnType,
} from "@/pages/pipelines/components/transform/utils";

const DIRECT_OPERATOR_ID = "direct";

const OperatorValue = styled.div`
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: ${TRANSFORM_GAP}px;

  width: 100%;
  min-width: 0;

  @container (max-width: ${TRANSFORM_CONDITION_STACK_WIDTH}px) {
    grid-template-columns: minmax(0, 1fr);
  }
`;

const lowerFirst = (value: string): string => value.charAt(0).toLowerCase() + value.slice(1);

interface PipelineTransformFieldsConditionRowProps {
  expr: TransformExpr;
  path: string;
  gutter: ReactNode;
  action: ReactNode;
  onChange: (expr: TransformExpr) => void;
}

const PipelineTransformFieldsConditionRow = ({
  expr,
  path,
  gutter,
  action,
  onChange,
}: PipelineTransformFieldsConditionRowProps) => {
  const editor = usePipelineTransformFieldsEditor();
  const { functionsByName } = usePipelineTransformFieldsEnvironment();
  const { root, calls } = getTransformChain(expr);
  const column = root.kind === TransformExprKind.COLUMN ? root.name : undefined;
  const columnType =
    column === undefined ? undefined : getTransformColumnType(editor.columns, column);
  const call = calls.length === 1 ? calls[0] : undefined;
  const fn = call ? functionsByName.get(call.fn) : undefined;
  const operators = getCompatibleTransformFunctions(columnType, false, functionsByName).filter(
    isTransformConditionOperator,
  );
  const listed =
    fn && !operators.some((candidate) => candidate.name === fn.name)
      ? [...operators, fn]
      : operators;
  const operatorOptions: SelectInputOption[] = [
    ...(columnType === "bool"
      ? [{ id: DIRECT_OPERATOR_ID, label: "is true", value: DIRECT_OPERATOR_ID }]
      : []),
    ...listed.map((candidate) => {
      const option = createTransformFunctionOption(candidate);
      return { ...option, label: lowerFirst(option.label) };
    }),
  ];
  const selectedOperatorId = call ? call.fn : columnType === "bool" ? DIRECT_OPERATOR_ID : "";
  const valueSpec = fn ? getTransformArgumentSpec(fn, 1) : undefined;
  const value = call?.args[0] ?? TRANSFORM_EMPTY_EXPR;

  const setColumn = (next: TransformLeafExpr) => {
    if (next.kind !== TransformExprKind.COLUMN) return onChange(next);
    const keepsOperator =
      call === undefined ||
      getCompatibleTransformFunctions(
        getTransformColumnType(editor.columns, next.name),
        false,
        functionsByName,
      ).some((candidate) => candidate.name === call.fn);
    onChange(createTransformChainExpr(next, keepsOperator ? calls : []));
  };
  const setOperator = (id: string) =>
    onChange(
      id === DIRECT_OPERATOR_ID
        ? root
        : createTransformChainExpr(root, [
            createTransformChainCall(id, call, columnType, functionsByName, editor.columns, column),
          ]),
    );
  const setValue = (next: TransformLeafExpr) => {
    if (!call) return;
    onChange(
      createTransformChainExpr(root, [
        { ...call, args: call.args.map((arg, slot) => (slot === 0 ? next : arg)) },
      ]),
    );
  };

  return (
    <PipelineTransformFieldsRow
      variant={PipelineTransformFieldsRowVariant.COND}
      gutter={gutter}
      action={action}
    >
      <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.SMALL} fillWidth>
        <PipelineTransformFieldsLeaf
          expr={root}
          onChange={setColumn}
          placeholder="Choose a column"
          isColumnOnly
          isError={editor.errors.has(call ? getTransformInputPath(path, call) : path)}
        />
        <OperatorValue>
          <SelectInput
            options={operatorOptions}
            value={operatorOptions.find((option) => option.id === selectedOperatorId) ?? null}
            onChange={(option) => setOperator(option.id)}
            onReset={() => onChange(root)}
            placeholder="Choose an operator"
            variant={InputVariant.TERTIARY}
            size={InputSize.MEDIUM}
            error={call && editor.errors.has(path) ? TRANSFORM_SELECT_ERROR_MARK : undefined}
            isDisabled={editor.isDisabled || column === undefined}
            fillWidth
          />
          {fn && call && valueSpec && value.kind !== TransformExprKind.CALL && (
            <PipelineTransformFieldsLeaf
              expr={value}
              onChange={setValue}
              placeholder="Value"
              logicalTypes={getTransformArgumentTypes(fn, 1, columnType)}
              isLiteralOnly={valueSpec.isLiteral}
              isColumnOnly={valueSpec.isColumn}
              isOptional={valueSpec.isOptional}
              isError={editor.errors.has(getTransformArgumentPath(path, call, 0))}
            />
          )}
        </OperatorValue>
      </FlexWrapper>
    </PipelineTransformFieldsRow>
  );
};

export default PipelineTransformFieldsConditionRow;
