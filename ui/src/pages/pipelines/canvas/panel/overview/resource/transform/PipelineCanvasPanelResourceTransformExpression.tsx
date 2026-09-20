import { create } from "@bufbuild/protobuf";
import { PlusIcon, XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import type { ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import {
  ListTransformFunctionsRequestSchema,
  type TransformFunction,
} from "@/gen/ingestion/v1/transformations_pb";

import type { TransformFunctionCall } from "@/pages/pipelines/canvas/panel/overview/resource/transform/types";
import { getTransformExpressionType } from "@/pages/pipelines/canvas/panel/overview/resource/transform/utils";

import { useListTransformFunctionsQuery } from "@/api/queries/transforms";

interface TransformFunctionCallRowProps {
  call: TransformFunctionCall;
  inputType: ResourceColumn["logicalType"];
  functionsByName: Map<string, TransformFunction>;
  onChange: (call: TransformFunctionCall) => void;
  onRemove: () => void;
  isDisabled: boolean;
}

/** One function in the expression. Its options are the catalog narrowed to the type flowing in. */
const TransformFunctionCallRow = ({
  call,
  inputType,
  functionsByName,
  onChange,
  onRemove,
  isDisabled,
}: TransformFunctionCallRowProps) => {
  const { data } = useListTransformFunctionsQuery({
    input: create(ListTransformFunctionsRequestSchema, { logicalType: inputType }),
    options: { enabled: inputType !== "" },
  });
  const options: SelectInputOption[] = (data?.functions ?? []).map((fn) => ({
    id: fn.name,
    label: fn.name,
    value: fn.name,
  }));
  const selected = options.find((option) => option.id === call.name) ?? null;
  const literalArg = functionsByName.get(call.name)?.args.find((arg) => arg.isLiteral);

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={8} fillWidth>
        <FlexItem grow={1} minWidth={0}>
          <SelectInput
            options={options}
            value={selected}
            onChange={(option) => onChange({ name: option.value as string, literal: "" })}
            variant={InputVariant.TERTIARY}
            placeholder={inputType === "" ? "Nothing applies here" : "Choose a function"}
            size={InputSize.LARGE}
            isDisabled={isDisabled || inputType === ""}
            fillWidth
          />
        </FlexItem>
        <Button
          icon={XIcon}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.LARGE}
          onClick={onRemove}
          isDisabled={isDisabled}
          ariaLabel="Remove function"
        />
      </FlexWrapper>
      {literalArg && (
        <TextInput
          label={literalArg.name}
          value={call.literal}
          onChange={(literal) => onChange({ ...call, literal })}
          placeholder={literalArg.isOptional ? "Optional" : ""}
          variant={InputVariant.TERTIARY}
          size={InputSize.LARGE}
          isDisabled={isDisabled}
          fillWidth
        />
      )}
    </FlexWrapper>
  );
};

interface PipelineCanvasPanelResourceTransformExpressionProps {
  columnType: ResourceColumn["logicalType"];
  expression: TransformFunctionCall[];
  functionsByName: Map<string, TransformFunction>;
  onChange: (expression: TransformFunctionCall[]) => void;
  isDisabled?: boolean;
}

/**
 * The functions applied to the column, in order. Each one sees the type the
 * previous one returns, so only functions that apply are offered.
 */
const PipelineCanvasPanelResourceTransformExpression = ({
  columnType,
  expression,
  functionsByName,
  onChange,
  isDisabled = false,
}: PipelineCanvasPanelResourceTransformExpressionProps) => {
  const resultType = getTransformExpressionType(columnType, expression, functionsByName);
  const isComplete = expression.every((call) => call.name !== "");

  const handleCallChange = (index: number, call: TransformFunctionCall) => {
    // A changed function changes the type flowing on, so later ones no longer apply.
    onChange([...expression.slice(0, index), call]);
  };

  const handleRemove = (index: number) => {
    onChange(expression.slice(0, index));
  };

  const handleAdd = () => {
    onChange([...expression, { name: "", literal: "" }]);
  };

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
      <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
        Expression
      </Text>
      {expression.map((call, index) => (
        <TransformFunctionCallRow
          key={expression
            .slice(0, index + 1)
            .map((prefix) => prefix.name)
            .join(">")}
          call={call}
          inputType={getTransformExpressionType(
            columnType,
            expression.slice(0, index),
            functionsByName,
          )}
          functionsByName={functionsByName}
          onChange={(next) => handleCallChange(index, next)}
          onRemove={() => handleRemove(index)}
          isDisabled={isDisabled}
        />
      ))}
      <FlexWrapper alignItems={AlignItems.CENTER} fillWidth>
        <Button
          label="Add function"
          icon={PlusIcon}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.MEDIUM}
          onClick={handleAdd}
          isDisabled={isDisabled || columnType === "" || !isComplete || resultType === ""}
        />
      </FlexWrapper>
    </FlexWrapper>
  );
};

export default PipelineCanvasPanelResourceTransformExpression;
