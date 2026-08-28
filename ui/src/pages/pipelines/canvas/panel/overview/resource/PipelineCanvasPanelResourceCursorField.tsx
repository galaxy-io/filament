import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

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

  const selectOptions: SelectInputOption[] = options.map((column) => ({
    id: column.name,
    label: column.name,
    value: column.name,
  }));

  return (
    <SelectInput
      label="Cursor"
      options={selectOptions}
      value={selectOptions.find((option) => option.value === value) ?? null}
      onChange={(option) => onChange(option.value as string)}
      placeholder="Select a column..."
      size={InputSize.LARGE}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default PipelineCanvasPanelResourceCursorField;
