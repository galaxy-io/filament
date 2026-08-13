import { styled } from "@linaria/react";

import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { ReadMode } from "@/gen/ingestion/v1/common_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import { CREATE_PIPELINE_MODAL_CURSOR_DROPDOWN_WIDTH } from "@/pages/pipelines/components/create/constants";
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
        onChange={(option) =>
          dispatch({
            type: CreatePipelineModalActionType.SET_RESOURCE_CURSOR,
            payload: {
              sinkId: activeSinkId,
              resource: row.name,
              cursorField: option.value as string,
            },
          })
        }
        placeholder="Select a column..."
        variant={InputVariant.TERTIARY}
        dropdownWidth={CREATE_PIPELINE_MODAL_CURSOR_DROPDOWN_WIDTH}
        fillWidth
      />
    </CellWrapper>
  );
};

export default CreatePipelineModalResourcesCursorCell;
