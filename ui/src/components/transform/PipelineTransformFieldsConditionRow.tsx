import type { ReactNode } from "react";

import { styled } from "@linaria/react";
import { XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import {
  TRANSFORM_CONDITION_STACK_WIDTH,
  TRANSFORM_GAP,
  TRANSFORM_SELECT_ERROR_MARK,
} from "@/components/transform/constants";
import { chainOf, chainToExpr, EMPTY_EXPR } from "@/components/transform/grammar/chain";
import { isTransformConditionOperator } from "@/components/transform/grammar/groups";
import { getTransformArgumentPath } from "@/components/transform/grammar/paths";
import { createTransformFunctionOption } from "@/components/transform/PipelineTransformFieldsFunctionPicker";
import PipelineTransformFieldsLeaf from "@/components/transform/PipelineTransformFieldsLeaf";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/components/transform/PipelineTransformFieldsRow";
import { type TransformExpr, TransformExprKind } from "@/components/transform/types";
import {
  createTransformChainCall,
  getCompatibleTransformFunctions,
  getTransformArgumentSpec,
  getTransformArgumentTypes,
  getTransformColumnType,
} from "@/components/transform/utils";

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
  onChange: (expr: TransformExpr) => void;
  onRemove?: () => void;
}

const PipelineTransformFieldsConditionRow = ({
  expr,
  path,
  gutter,
  onChange,
  onRemove,
}: PipelineTransformFieldsConditionRowProps) => {
  const editor = usePipelineTransformFieldsEditor();
  const { functionsByName } = usePipelineTransformFieldsEnvironment();
  const { root, calls } = chainOf(expr);
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
  const value = call?.args[0] ?? EMPTY_EXPR;

  const handleColumnChange = (next: TransformExpr) => {
    if (next.kind !== TransformExprKind.COLUMN) return onChange(next);
    const keepsOperator =
      call === undefined ||
      getCompatibleTransformFunctions(
        getTransformColumnType(editor.columns, next.name),
        false,
        functionsByName,
      ).some((candidate) => candidate.name === call.fn);
    onChange(chainToExpr(next, keepsOperator ? calls : []));
  };
  const handleOperatorChange = (id: string) =>
    onChange(
      id === DIRECT_OPERATOR_ID
        ? root
        : chainToExpr(root, [
            createTransformChainCall(id, call, columnType, functionsByName, editor.columns, column),
          ]),
    );

  return (
    <PipelineTransformFieldsRow
      variant={PipelineTransformFieldsRowVariant.COND}
      gutter={gutter}
      action={
        onRemove ? (
          <Button
            icon={XIcon}
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.MEDIUM}
            onClick={onRemove}
            isDisabled={editor.isDisabled}
            ariaLabel="Remove condition"
          />
        ) : undefined
      }
    >
      <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.SMALL} fillWidth>
        <PipelineTransformFieldsLeaf
          expr={root}
          onChange={handleColumnChange}
          placeholder="Choose a column"
          isColumnOnly
          isError={editor.errors.has(call ? `${path}.${call.fn}[0]` : path)}
        />
        <OperatorValue>
          <SelectInput
            options={operatorOptions}
            value={operatorOptions.find((option) => option.id === selectedOperatorId) ?? null}
            onChange={(option) => handleOperatorChange(option.id)}
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
              onChange={(next) => onChange(chainToExpr(root, [{ ...call, args: [next] }]))}
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
