import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import { usePipelineTransformFieldsEditor } from "@/components/transform/PipelineTransformFieldsProvider";

const ALL_ROWS_ID = "all";
const MATCHING_ROWS_ID = "matching";

const SCOPE_OPTIONS: SelectInputOption[] = [
  { id: ALL_ROWS_ID, label: "All rows", value: ALL_ROWS_ID },
  { id: MATCHING_ROWS_ID, label: "Matching rows", value: MATCHING_ROWS_ID },
];

interface PipelineTransformFieldsConditionScopeProps {
  isMatching: boolean;
  onChange: (isMatching: boolean) => void;
}

const PipelineTransformFieldsConditionScope = ({
  isMatching,
  onChange,
}: PipelineTransformFieldsConditionScopeProps) => {
  const { isDisabled } = usePipelineTransformFieldsEditor();
  return (
    <SelectInput
      options={SCOPE_OPTIONS}
      value={SCOPE_OPTIONS[isMatching ? 1 : 0]}
      onChange={(option) => onChange(option.id === MATCHING_ROWS_ID)}
      onReset={() => onChange(false)}
      variant={InputVariant.TERTIARY}
      size={InputSize.MEDIUM}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default PipelineTransformFieldsConditionScope;
