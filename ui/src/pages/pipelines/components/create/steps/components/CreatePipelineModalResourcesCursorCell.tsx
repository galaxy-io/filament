import type { FC } from "react";

import SelectInput, { SelectInputVariant } from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { ReadMode } from "@/gen/ingestion/v1/common_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";
import { getCursorSelectOptions } from "@/pages/pipelines/components/resource/utils";

interface CreatePipelineModalResourcesCursorCellProps {
  row: CreatePipelineModalResourceRow;
}

const CreatePipelineModalResourcesCursorCell: FC<CreatePipelineModalResourcesCursorCellProps> = ({
  row,
}) => {
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

  const options = getCursorSelectOptions(row.cursorOptions);

  const handleCursorChange = (id: string | null) => {
    if (id === null) return;
    dispatch({
      type: CreatePipelineModalActionType.SET_RESOURCE_CURSOR,
      payload: { sinkId: activeSinkId, resource: row.name, cursorField: id },
    });
  };

  return (
    <Box fillWidth minWidth={0}>
      <SelectInput
        options={options}
        value={row.cursorField || null}
        onChange={handleCursorChange}
        placeholder="Select a column..."
        variant={SelectInputVariant.TERTIARY}
        fillWidth
      />
    </Box>
  );
};

export default CreatePipelineModalResourcesCursorCell;
