import type { FC } from "react";

import SelectInput, { SelectInputVariant } from "@galaxy-io/dls/inputs/SelectInput";

import type { ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";

import { getCursorSelectOptions } from "@/pages/pipelines/components/resource/utils";

interface PipelineCanvasPanelResourceCursorFieldProps {
  value: ResourceColumn["name"];
  options: ResourceColumn[];
  isDisabled: boolean;
  error?: string;
  onChange: (field: ResourceColumn["name"]) => void;
}

const PipelineCanvasPanelResourceCursorField: FC<PipelineCanvasPanelResourceCursorFieldProps> = ({
  value,
  options,
  isDisabled,
  error,
  onChange,
}) => {
  if (!options.length) {
    return null;
  }

  const selectOptions = getCursorSelectOptions(options);

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
      error={error}
      fillWidth
    />
  );
};

export default PipelineCanvasPanelResourceCursorField;
