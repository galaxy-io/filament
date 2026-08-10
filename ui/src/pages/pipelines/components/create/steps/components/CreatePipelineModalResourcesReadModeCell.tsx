import { styled } from "@linaria/react";

import SelectInput, {
  type SelectInputOption,
  SelectInputVariant,
} from "@galaxy-io/dls/inputs/SelectInput";

import type { ReadMode } from "@/gen/ingestion/v1/common_pb";

import { READ_MODE_TO_LABEL_MAP } from "@/pages/pipelines/components/create/constants";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";

const CellWrapper = styled.div`
  width: 100%;
  min-width: 0;
`;

interface CreatePipelineModalResourcesReadModeCellProps {
  row: CreatePipelineModalResourceRow;
  onChange: (resource: string, readMode: ReadMode) => void;
}

const CreatePipelineModalResourcesReadModeCell = ({
  row,
  onChange,
}: CreatePipelineModalResourcesReadModeCellProps) => {
  const options: SelectInputOption[] = row.readModeOptions.map((mode) => ({
    id: String(mode),
    label: READ_MODE_TO_LABEL_MAP[mode],
    value: mode,
  }));

  const selectedOption = options.find((option) => option.value === row.readMode) ?? null;

  return (
    <CellWrapper>
      <SelectInput
        options={options}
        value={selectedOption}
        onChange={(option) => onChange(row.name, option.value as ReadMode)}
        variant={SelectInputVariant.SECONDARY}
        dropdownWidth={220}
        isDisabled={!row.isSelected}
        fillWidth
      />
    </CellWrapper>
  );
};

export default CreatePipelineModalResourcesReadModeCell;
