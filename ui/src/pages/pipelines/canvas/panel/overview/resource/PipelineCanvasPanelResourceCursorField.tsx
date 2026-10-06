import SelectInput, {
  SelectInputVariant,
  type SelectOption,
} from "@galaxy-io/dls/inputs/SelectInput";

import type { ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";

interface PipelineCanvasPanelResourceCursorFieldProps {
  value: ResourceColumn["name"];
  options: ResourceColumn[];
  isDisabled: boolean;
  onChange: (field: ResourceColumn["name"]) => void;
}

const PipelineCanvasPanelResourceCursorField = ({
  value,
  options,
  isDisabled,
  onChange,
}: PipelineCanvasPanelResourceCursorFieldProps) => {
  if (!options.length) {
    return null;
  }

  const selectOptions: SelectOption[] = options.map((column) => ({
    id: column.name,
    label: column.name,
  }));

  return (
    <SelectInput
      label="Cursor"
      options={selectOptions}
      value={value || null}
      onChange={(id) => {
        if (id !== null) onChange(id);
      }}
      placeholder="Select a column..."
      isDisabled={isDisabled}
      variant={SelectInputVariant.TERTIARY}
      fillWidth
    />
  );
};

export default PipelineCanvasPanelResourceCursorField;
