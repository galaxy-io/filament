import { CopyIcon, PencilSimpleIcon, TrashSimpleIcon } from "@phosphor-icons/react";

import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import {
  TRANSFORM_LITERAL_KIND_TO_ICON_MAP,
  TRANSFORM_LITERAL_KIND_TO_LABEL_MAP,
  TRANSFORM_LITERAL_KINDS,
  TRANSFORM_SELECT_ERROR_MARK,
  TRANSFORM_SELECT_SEARCH_THRESHOLD,
} from "@/pages/pipelines/components/transform/constants";
import { getTransformFunctionChoices } from "@/pages/pipelines/components/transform/grammar/catalog";
import PipelineTransformFieldsOptionIcon from "@/pages/pipelines/components/transform/PipelineTransformFieldsOptionIcon";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import {
  type TransformAction,
  TransformActionKind,
  type TransformLeafExpr,
} from "@/pages/pipelines/components/transform/types";
import {
  createTransformFunctionOption,
  filterTransformOptions,
  getTransformActionId,
} from "@/pages/pipelines/components/transform/utils";

const COPY_ACTION: TransformAction = { kind: TransformActionKind.COPY };

const createActionOption = (
  action: TransformAction,
  label: string,
  icon?: SelectInputOption["icon"],
): SelectInputOption => ({ id: getTransformActionId(action), label, value: action, icon });

const KIND_OPTIONS: SelectInputOption[] = [
  createActionOption(
    { kind: TransformActionKind.RENAME },
    "Rename",
    <PipelineTransformFieldsOptionIcon icon={PencilSimpleIcon} />,
  ),
  createActionOption(
    { kind: TransformActionKind.DROP },
    "Drop",
    <PipelineTransformFieldsOptionIcon icon={TrashSimpleIcon} />,
  ),
];
const COPY_OPTION = createActionOption(
  COPY_ACTION,
  "Duplicate",
  <PipelineTransformFieldsOptionIcon icon={CopyIcon} />,
);
const LITERAL_OPTIONS: SelectInputOption[] = TRANSFORM_LITERAL_KINDS.map((literalKind) =>
  createActionOption(
    { kind: TransformActionKind.LITERAL, literalKind },
    TRANSFORM_LITERAL_KIND_TO_LABEL_MAP[literalKind],
    <PipelineTransformFieldsOptionIcon icon={TRANSFORM_LITERAL_KIND_TO_ICON_MAP[literalKind]} />,
  ),
);

interface PipelineTransformFieldsActionPickerProps {
  root: TransformLeafExpr;
  rootType: string | undefined;
  action: TransformAction;
  offersKinds: boolean;
  onChange: (action: TransformAction) => void;
  isError: boolean;
}

const PipelineTransformFieldsActionPicker = ({
  root,
  rootType,
  action,
  offersKinds,
  onChange,
  isError,
}: PipelineTransformFieldsActionPickerProps) => {
  const { isDisabled } = usePipelineTransformFieldsEditor();
  const { functionsByName } = usePipelineTransformFieldsEnvironment();
  const currentFn = action.kind === TransformActionKind.FUNCTION ? action.fn : "";
  const options: SelectInputOption[] = [
    ...(offersKinds ? KIND_OPTIONS : []),
    COPY_OPTION,
    ...LITERAL_OPTIONS,
    ...getTransformFunctionChoices(root, rootType, currentFn, functionsByName).map((fn) => {
      const action: TransformAction = { kind: TransformActionKind.FUNCTION, fn: fn.name };
      return {
        ...createTransformFunctionOption(fn),
        id: getTransformActionId(action),
        value: action,
      };
    }),
  ];
  const selectedId = getTransformActionId(action);

  return (
    <SelectInput
      options={options}
      value={options.find((option) => option.id === selectedId) ?? null}
      onChange={(option) => onChange(option.value as TransformAction)}
      onReset={() => onChange(COPY_ACTION)}
      onSearch={
        options.length > TRANSFORM_SELECT_SEARCH_THRESHOLD ? filterTransformOptions : undefined
      }
      placeholder="Choose an action"
      variant={InputVariant.TERTIARY}
      size={InputSize.MEDIUM}
      error={isError ? TRANSFORM_SELECT_ERROR_MARK : undefined}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default PipelineTransformFieldsActionPicker;
