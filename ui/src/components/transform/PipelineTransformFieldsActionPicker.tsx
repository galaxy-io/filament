import { CopyIcon } from "@phosphor-icons/react";

import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import {
  TRANSFORM_ACTION_TO_LITERAL_KIND_MAP,
  TRANSFORM_COPY_ACTION,
  TRANSFORM_LITERAL_KIND_TO_ACTION_MAP,
  TRANSFORM_LITERAL_KIND_TO_ICON_MAP,
  TRANSFORM_LITERAL_KIND_TO_LABEL_MAP,
  TRANSFORM_LITERAL_KINDS,
  TRANSFORM_SELECT_ERROR_MARK,
  TRANSFORM_SELECT_SEARCH_THRESHOLD,
  TRANSFORM_STEP_KIND_TO_LABEL_MAP,
} from "@/components/transform/constants";
import { createTransformFunctionOption } from "@/components/transform/PipelineTransformFieldsFunctionPicker";
import PipelineTransformFieldsOptionIcon from "@/components/transform/PipelineTransformFieldsOptionIcon";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/components/transform/PipelineTransformFieldsProvider";
import {
  TransformExprKind,
  type TransformLeafExpr,
  type TransformLiteralKind,
  TransformStepKind,
} from "@/components/transform/types";
import {
  filterTransformOptions,
  getTransformExprType,
  getTransformFunctionChoices,
} from "@/components/transform/utils";

const KIND_OPTIONS: SelectInputOption[] = (
  [TransformStepKind.RENAME, TransformStepKind.DROP] as const
).map((kind) => ({ id: kind, label: TRANSFORM_STEP_KIND_TO_LABEL_MAP[kind], value: kind }));
const COPY_OPTION: SelectInputOption = {
  id: TRANSFORM_COPY_ACTION,
  label: "Duplicate",
  value: TRANSFORM_COPY_ACTION,
  icon: <PipelineTransformFieldsOptionIcon icon={CopyIcon} />,
};
const LITERAL_OPTIONS: SelectInputOption[] = TRANSFORM_LITERAL_KINDS.map((kind) => ({
  id: TRANSFORM_LITERAL_KIND_TO_ACTION_MAP[kind],
  label: TRANSFORM_LITERAL_KIND_TO_LABEL_MAP[kind],
  value: kind,
  icon: <PipelineTransformFieldsOptionIcon icon={TRANSFORM_LITERAL_KIND_TO_ICON_MAP[kind]} />,
}));

interface PipelineTransformFieldsActionPickerProps {
  root: TransformLeafExpr;
  rootPath: string;
  action: string;
  offersKinds: boolean;
  onKind: (kind: TransformStepKind.RENAME | TransformStepKind.DROP) => void;
  onCopy: () => void;
  onLiteral: (literalKind: TransformLiteralKind) => void;
  onFunction: (fn: TransformFunction["name"]) => void;
  isError: boolean;
}

const PipelineTransformFieldsActionPicker = ({
  root,
  rootPath,
  action,
  offersKinds,
  onKind,
  onCopy,
  onLiteral,
  onFunction,
  isError,
}: PipelineTransformFieldsActionPickerProps) => {
  const editor = usePipelineTransformFieldsEditor();
  const { functionsByName } = usePipelineTransformFieldsEnvironment();
  const rootType =
    root.kind === TransformExprKind.EMPTY
      ? undefined
      : getTransformExprType(root, rootPath, editor);
  const options: SelectInputOption[] = [
    ...(offersKinds ? KIND_OPTIONS : []),
    COPY_OPTION,
    ...LITERAL_OPTIONS,
    ...getTransformFunctionChoices(root, rootType, action, functionsByName).map(
      createTransformFunctionOption,
    ),
  ];

  return (
    <SelectInput
      options={options}
      value={options.find((option) => option.id === action) ?? null}
      onChange={(option) => {
        const literalKind = TRANSFORM_ACTION_TO_LITERAL_KIND_MAP.get(option.id);
        if (option.id === TransformStepKind.RENAME || option.id === TransformStepKind.DROP) {
          onKind(option.id);
        } else if (option.id === TRANSFORM_COPY_ACTION) {
          onCopy();
        } else if (literalKind !== undefined) {
          onLiteral(literalKind);
        } else {
          onFunction(option.id);
        }
      }}
      onReset={onCopy}
      onSearch={
        options.length > TRANSFORM_SELECT_SEARCH_THRESHOLD ? filterTransformOptions : undefined
      }
      placeholder="Choose an action"
      variant={InputVariant.TERTIARY}
      size={InputSize.MEDIUM}
      error={isError ? TRANSFORM_SELECT_ERROR_MARK : undefined}
      isDisabled={editor.isDisabled}
      fillWidth
    />
  );
};

export default PipelineTransformFieldsActionPicker;
