import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import {
  TRANSFORM_FUNCTION_TO_ICON_MAP,
  TRANSFORM_SELECT_ERROR_MARK,
  TRANSFORM_SELECT_SEARCH_THRESHOLD,
} from "@/components/transform/constants";
import PipelineTransformFieldsOptionIcon from "@/components/transform/PipelineTransformFieldsOptionIcon";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/components/transform/PipelineTransformFieldsProvider";
import { type TransformExpr, TransformExprKind } from "@/components/transform/types";
import {
  filterTransformOptions,
  getTransformExprType,
  getTransformFunctionChoices,
  isTransformExprComplete,
} from "@/components/transform/utils";

export const createTransformFunctionOption = (fn: TransformFunction): SelectInputOption => {
  const icon = TRANSFORM_FUNCTION_TO_ICON_MAP.get(fn.name);
  return {
    id: fn.name,
    label: fn.displayName || fn.name,
    value: fn.name,
    icon: icon ? <PipelineTransformFieldsOptionIcon icon={icon} /> : undefined,
  };
};

interface PipelineTransformFieldsFunctionPickerProps {
  input: TransformExpr;
  inputPath: string;
  fn: TransformFunction["name"];
  onChange: (fn: TransformFunction["name"]) => void;
  isError?: boolean;
}

const PipelineTransformFieldsFunctionPicker = ({
  input,
  inputPath,
  fn,
  onChange,
  isError = false,
}: PipelineTransformFieldsFunctionPickerProps) => {
  const editor = usePipelineTransformFieldsEditor();
  const { functionsByName } = usePipelineTransformFieldsEnvironment();
  const isInputComplete = isTransformExprComplete(input, functionsByName);
  const inputType = isInputComplete ? getTransformExprType(input, inputPath, editor) : undefined;
  const options = getTransformFunctionChoices(input, inputType, fn, functionsByName).map(
    createTransformFunctionOption,
  );
  const isInputPending = !isInputComplete && input.kind !== TransformExprKind.EMPTY;

  return (
    <SelectInput
      options={options}
      value={options.find((option) => option.id === fn) ?? null}
      onChange={(option) => onChange(option.id)}
      onReset={() => onChange("")}
      onSearch={
        options.length > TRANSFORM_SELECT_SEARCH_THRESHOLD ? filterTransformOptions : undefined
      }
      placeholder={isInputPending ? "Complete the input first" : "Choose a function"}
      variant={InputVariant.TERTIARY}
      size={InputSize.MEDIUM}
      error={isError ? TRANSFORM_SELECT_ERROR_MARK : undefined}
      isDisabled={editor.isDisabled || isInputPending}
      fillWidth
    />
  );
};

export default PipelineTransformFieldsFunctionPicker;
