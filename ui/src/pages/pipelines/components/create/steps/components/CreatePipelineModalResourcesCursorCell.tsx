import { styled } from "@linaria/react";

import SelectInput, {
  SelectInputVariant,
  type SelectOption,
} from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { ReadMode } from "@/gen/ingestion/v1/common_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";

const CellWrapper = styled.div`
  width: 100%;
  min-width: 0;
`;

interface CreatePipelineModalResourcesCursorCellProps {
  row: CreatePipelineModalResourceRow;
}

const CreatePipelineModalResourcesCursorCell = ({
  row,
}: CreatePipelineModalResourcesCursorCellProps) => {
  const { activeSinkId } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

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
        {row.managedIncremental ? "Source-managed" : "Auto"}
      </Text>
    );
  }

  const options: SelectOption[] = row.cursorOptions.map((column) => ({
    id: column.name,
    label: column.name,
  }));

  const handleCursorChange = (id: string | null) => {
    if (id === null) return;
    dispatch({
      type: CreatePipelineModalActionType.SET_RESOURCE_CURSOR,
      payload: { sinkId: activeSinkId, resource: row.name, cursorField: id },
    });
  };

  return (
    <CellWrapper>
      <SelectInput
        options={options}
        value={row.cursorField || null}
        onChange={handleCursorChange}
        placeholder="Select a column..."
        variant={SelectInputVariant.TERTIARY}
        fillWidth
      />
    </CellWrapper>
  );
};

export default CreatePipelineModalResourcesCursorCell;
