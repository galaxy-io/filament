import type { FC } from "react";

import { CopyIcon, PencilSimpleIcon, TrashSimpleIcon } from "@phosphor-icons/react";

import SelectInput, {
  SelectInputSize,
  SelectInputVariant,
  type SelectOption,
} from "@galaxy-io/dls/inputs/SelectInput";

import {
  TRANSFORM_LITERAL_KIND_TO_ICON_MAP,
  TRANSFORM_LITERAL_KIND_TO_LABEL_MAP,
  TRANSFORM_LITERAL_KINDS,
  TRANSFORM_SELECT_SEARCH_THRESHOLD,
} from "@/pages/pipelines/components/transform/constants";
import { getTransformFunctionChoices } from "@/pages/pipelines/components/transform/grammar/catalog";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import {
  type TransformAction,
  TransformActionKind,
  type TransformLeafExpr,
  type TransformSelectEntry,
} from "@/pages/pipelines/components/transform/types";
import {
  createTransformFunctionOption,
  createTransformSelectModel,
  getTransformActionId,
} from "@/pages/pipelines/components/transform/utils";

const COPY_ACTION: TransformAction = { kind: TransformActionKind.COPY };

const createActionEntry = (
  action: TransformAction,
  label: string,
  icon?: SelectOption["icon"],
): TransformSelectEntry<TransformAction> => ({
  option: { id: getTransformActionId(action), label, icon },
  payload: action,
});

const KIND_ENTRIES = [
  createActionEntry({ kind: TransformActionKind.RENAME }, "Rename", PencilSimpleIcon),
  createActionEntry({ kind: TransformActionKind.DROP }, "Drop", TrashSimpleIcon),
];
const COPY_ENTRY = createActionEntry(COPY_ACTION, "Duplicate", CopyIcon);
const LITERAL_ENTRIES = TRANSFORM_LITERAL_KINDS.map((literalKind) =>
  createActionEntry(
    { kind: TransformActionKind.LITERAL, literalKind },
    TRANSFORM_LITERAL_KIND_TO_LABEL_MAP[literalKind],
    TRANSFORM_LITERAL_KIND_TO_ICON_MAP[literalKind],
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

const PipelineTransformFieldsActionPicker: FC<PipelineTransformFieldsActionPickerProps> = ({
  root,
  rootType,
  action,
  offersKinds,
  onChange,
  isError,
}) => {
  const { isDisabled } = usePipelineTransformFieldsEditor();
  const { functionsByName } = usePipelineTransformFieldsEnvironment();
  const currentFn = action.kind === TransformActionKind.FUNCTION ? action.fn : "";
  const { options, payloadById } = createTransformSelectModel([
    ...(offersKinds ? KIND_ENTRIES : []),
    COPY_ENTRY,
    ...LITERAL_ENTRIES,
    ...getTransformFunctionChoices(root, rootType, currentFn, functionsByName).map((fn) => {
      const payload: TransformAction = { kind: TransformActionKind.FUNCTION, fn: fn.name };
      return {
        option: { ...createTransformFunctionOption(fn), id: getTransformActionId(payload) },
        payload,
      };
    }),
  ]);

  return (
    <SelectInput
      ariaLabel="Action"
      options={options}
      value={getTransformActionId(action)}
      onChange={(id) => onChange((id === null ? undefined : payloadById.get(id)) ?? COPY_ACTION)}
      isClearable
      isSearchable={!isDisabled && options.length > TRANSFORM_SELECT_SEARCH_THRESHOLD}
      placeholder="Choose an action..."
      variant={SelectInputVariant.TERTIARY}
      size={SelectInputSize.MEDIUM}
      isError={isError}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default PipelineTransformFieldsActionPicker;
