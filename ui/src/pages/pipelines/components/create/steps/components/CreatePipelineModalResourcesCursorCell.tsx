import { styled } from "@linaria/react";

import SelectInput, {
  type SelectInputOption,
  SelectInputVariant,
} from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { ReadMode } from "@/gen/ingestion/v1/common_pb";

import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";

const CellWrapper = styled.div`
  width: 100%;
  min-width: 0;
`;

interface CreatePipelineModalResourcesCursorCellProps {
  row: CreatePipelineModalResourceRow;
  onChange: (resource: string, cursorField: string) => void;
}

const CreatePipelineModalResourcesCursorCell = ({
  row,
  onChange,
}: CreatePipelineModalResourcesCursorCellProps) => {
  if (!row.isSelected || row.readMode !== ReadMode.INCREMENTAL) {
    return (
      <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
        —
      </Text>
    );
  }

  if (!row.cursorOptions.length) {
    return (
      <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
        Auto
      </Text>
    );
  }

  const options: SelectInputOption[] = row.cursorOptions.map((column) => ({
    id: column.name,
    label: column.name,
    value: column.name,
  }));

  const selectedOption = options.find((option) => option.value === row.cursorField) ?? null;

  return (
    <CellWrapper>
      <SelectInput
        options={options}
        value={selectedOption}
        onChange={(option) => onChange(row.name, option.value as string)}
        placeholder="Select a column..."
        variant={SelectInputVariant.SECONDARY}
        dropdownWidth={260}
        fillWidth
      />
    </CellWrapper>
  );
};

export default CreatePipelineModalResourcesCursorCell;
