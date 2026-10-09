import ToggleInput, { type ToggleOption } from "@galaxy-io/dls/inputs/ToggleInput";

import { usePipelineTransformFieldsEditor } from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";

const ALL_ROWS_ID = "all";
const MATCHING_ROWS_ID = "matching";

const SCOPE_OPTIONS: ToggleOption[] = [
  { id: ALL_ROWS_ID, label: "All rows" },
  { id: MATCHING_ROWS_ID, label: "Matching rows" },
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
    <ToggleInput
      options={SCOPE_OPTIONS}
      value={isMatching ? MATCHING_ROWS_ID : ALL_ROWS_ID}
      onChange={(id) => onChange(id === MATCHING_ROWS_ID)}
      ariaLabel="Rows"
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default PipelineTransformFieldsConditionScope;
