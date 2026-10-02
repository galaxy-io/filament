import { XIcon } from "@phosphor-icons/react";

import Input, { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import {
  TRANSFORM_LITERAL_KIND_TO_ICON_MAP,
  TRANSFORM_SELECT_ERROR_MARK,
} from "@/pages/pipelines/components/transform/constants";
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
  const trailing = onClear ? { icon: XIcon, onClick: onClear } : undefined;

  if (expr.literalKind === TransformLiteralKind.BOOLEAN) {
    return (
      <SelectInput
        options={BOOLEAN_OPTIONS}
        value={BOOLEAN_OPTIONS.find((option) => option.id === expr.value) ?? null}
        onChange={(option) => onChange({ ...expr, value: option.id })}
        onReset={onClear ?? (() => onChange({ ...expr, value: null }))}
        placeholder={placeholder}
        variant={InputVariant.TERTIARY}
        size={InputSize.MEDIUM}
        error={isError ? TRANSFORM_SELECT_ERROR_MARK : undefined}
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
      <Input<string>
        type="text"
        parse={(value) => value}
        value={expr.value ?? ""}
        onChange={(value) => onChange({ ...expr, value: value === "" ? null : value })}
        placeholder={placeholder}
        leading={{ icon: TRANSFORM_LITERAL_KIND_TO_ICON_MAP[TransformLiteralKind.NUMBER] }}
        trailing={trailing}
        isError={isError || numberError !== null}
        variant={InputVariant.TERTIARY}
        size={InputSize.MEDIUM}
        isDisabled={isDisabled}
        isMonospace
        fillWidth
      />
    );
  }

  return (
    <TextInput
      value={expr.value ?? ""}
      onChange={(value) => onChange({ ...expr, value })}
      placeholder={placeholder}
      leading={{ icon: TRANSFORM_LITERAL_KIND_TO_ICON_MAP[TransformLiteralKind.STRING] }}
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
