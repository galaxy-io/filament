import { XIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import {
  INPUT_SIZE_TO_BUTTON_SIZE_MAP,
  InputSize,
  InputVariant,
} from "@galaxy-io/dls/inputs/Input";
import SelectInput, {
  SelectInputSize,
  SelectInputVariant,
} from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import { TRANSFORM_LITERAL_KIND_TO_ICON_MAP } from "@/pages/pipelines/components/transform/constants";
import { isTransformIntegerOnly } from "@/pages/pipelines/components/transform/grammar/catalog";
import { usePipelineTransformFieldsEditor } from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import {
  type TransformLiteralExpr,
  TransformLiteralKind,
} from "@/pages/pipelines/components/transform/types";
import {
  createTransformBooleanOptions,
  getTransformNumberError,
} from "@/pages/pipelines/components/transform/utils";

const BOOLEAN_OPTIONS = createTransformBooleanOptions();

interface PipelineTransformFieldsLiteralProps {
  expr: TransformLiteralExpr;
  logicalTypes: string[];
  placeholder?: string;
  isError: boolean;
  onChange: (expr: TransformLiteralExpr) => void;
  onClear?: () => void;
}

const PipelineTransformFieldsLiteral = ({
  expr,
  logicalTypes,
  placeholder,
  isError,
  onChange,
  onClear,
}: PipelineTransformFieldsLiteralProps) => {
  const { isDisabled } = usePipelineTransformFieldsEditor();
  const trailing = onClear ? (
    <Button
      icon={XIcon}
      ariaLabel="Clear value"
      tooltip="Clear value"
      variant={ButtonVariant.TERTIARY}
      size={INPUT_SIZE_TO_BUTTON_SIZE_MAP[InputSize.SMALL]}
      isDisabled={isDisabled}
      onClick={onClear}
    />
  ) : undefined;

  const handleBooleanChange = (id: string | null) => {
    if (id !== null) onChange({ ...expr, value: id });
    else if (onClear) onClear();
    else onChange({ ...expr, value: null });
  };

  if (expr.literalKind === TransformLiteralKind.BOOLEAN) {
    return (
      <SelectInput
        ariaLabel="Value"
        options={BOOLEAN_OPTIONS}
        value={expr.value}
        onChange={handleBooleanChange}
        isClearable
        placeholder={placeholder}
        variant={SelectInputVariant.TERTIARY}
        size={SelectInputSize.MEDIUM}
        isError={isError}
        isDisabled={isDisabled}
        fillWidth
      />
    );
  }

  if (expr.literalKind === TransformLiteralKind.NUMBER) {
    const numberError =
      expr.value === null
        ? null
        : getTransformNumberError(expr.value, isTransformIntegerOnly(logicalTypes));
    return (
      <TextInput
        ariaLabel="Value"
        inputMode="decimal"
        value={expr.value ?? ""}
        onChange={(value) => onChange({ ...expr, value: value === "" ? null : value })}
        placeholder={placeholder}
        icon={TRANSFORM_LITERAL_KIND_TO_ICON_MAP[TransformLiteralKind.NUMBER]}
        trailing={trailing}
        isError={isError || numberError !== null}
        variant={InputVariant.TERTIARY}
        size={InputSize.MEDIUM}
        isDisabled={isDisabled}
        family={FontFamily.MONO}
        fillWidth
      />
    );
  }

  return (
    <TextInput
      ariaLabel="Value"
      value={expr.value ?? ""}
      onChange={(value) => onChange({ ...expr, value })}
      placeholder={placeholder}
      icon={TRANSFORM_LITERAL_KIND_TO_ICON_MAP[TransformLiteralKind.STRING]}
      trailing={trailing}
      isError={isError}
      variant={InputVariant.TERTIARY}
      size={InputSize.MEDIUM}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default PipelineTransformFieldsLiteral;
